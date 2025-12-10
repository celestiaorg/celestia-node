package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/v6/pkg/da"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

// Listener is responsible for listening to Core for
// new block events and converting new Core blocks into
// the main data structure used in the Celestia DA network:
// `ExtendedHeader`. After digesting the Core block, extending
// it, and generating the `ExtendedHeader`, the Listener
// broadcasts the new `ExtendedHeader` to the header-sub gossipsub
// network.
type Listener struct {
	fetcher *BlockFetcher

	construct          header.ConstructFn
	store              *store.Store
	availabilityWindow time.Duration
	archival           bool

	headerBroadcaster libhead.Broadcaster[*header.ExtendedHeader]
	hashBroadcaster   shrexsub.BroadcastFn

	metrics *listenerMetrics

	chainID string

	listenerTimeout time.Duration
	cancel          context.CancelFunc
	closed          chan struct{}
}

func NewListener(
	bcast libhead.Broadcaster[*header.ExtendedHeader],
	fetcher *BlockFetcher,
	hashBroadcaster shrexsub.BroadcastFn,
	construct header.ConstructFn,
	store *store.Store,
	blocktime time.Duration,
	opts ...Option,
) (*Listener, error) {
	p := defaultParams()
	for _, opt := range opts {
		opt(&p)
	}

	var (
		metrics *listenerMetrics
		err     error
	)
	if p.metrics {
		metrics, err = newListenerMetrics()
		if err != nil {
			return nil, err
		}
	}

	return &Listener{
		fetcher:            fetcher,
		headerBroadcaster:  bcast,
		hashBroadcaster:    hashBroadcaster,
		construct:          construct,
		store:              store,
		availabilityWindow: p.availabilityWindow,
		archival:           p.archival,
		listenerTimeout:    5 * blocktime,
		metrics:            metrics,
		chainID:            p.chainID,
	}, nil
}

// Start kicks off the Listener listener loop.
func (cl *Listener) Start(context.Context) error {
	if cl.cancel != nil {
		return fmt.Errorf("listener: already started")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cl.cancel = cancel
	cl.closed = make(chan struct{})

	subs, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}

	go cl.listen(ctx, subs)
	return nil
}

// Stop stops the listener loop.
func (cl *Listener) Stop(ctx context.Context) error {
	cl.cancel()
	select {
	case <-cl.closed:
		cl.cancel = nil
		cl.closed = nil
	case <-ctx.Done():
		return ctx.Err()
	}

	err := cl.metrics.Close()
	if err != nil {
		log.Warnw("listener: closing metrics", "err", err)
	}
	return nil
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *Listener) listen(ctx context.Context, sub <-chan SignedBlock) {
	defer close(cl.closed)
	defer log.Info("listener: listening stopped")
	timeout := time.NewTimer(cl.listenerTimeout)
	defer timeout.Stop()
	for {
		select {
		case b, ok := <-sub:
			if !ok {
				log.Error("underlying subscription was closed")
				return
			}

			if cl.chainID != "" && b.Header.ChainID != cl.chainID {
				// stop node if there is a critical issue with the block subscription
				panic(fmt.Sprintf("listener: received block with unexpected chain ID: expected %s,"+
					" received %s. blockHeight: %d blockHash: %x.",
					cl.chainID, b.Header.ChainID, b.Header.Height, b.Header.Hash()),
				)
			}

			log.Debugw("listener: new block from core", "height", b.Header.Height)

			err := cl.handleNewSignedBlock(ctx, b)
			if err != nil {
				log.Errorw("listener: handling new block msg",
					"height", b.Header.Height,
					"hash", b.Header.Hash().String(),
					"err", err)
			}
		case <-timeout.C:
			cl.metrics.subscriptionStuck(ctx)
			log.Error("underlying subscription is stuck")
		case <-ctx.Done():
			return
		}
		timeout.Reset(cl.listenerTimeout)
	}
}

func (cl *Listener) handleNewSignedBlock(ctx context.Context, b SignedBlock) error {
	var err error

	ctx, span := tracer.Start(ctx, "listener/handleNewSignedBlock")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(
		attribute.Int64("height", b.Header.Height),
	)

	eds, err := da.ConstructEDS(b.Data.Txs.ToSliceOfBytes(), b.Header.Version.App, -1)
	if err != nil {
		return fmt.Errorf("extending block data: %w", err)
	}

	// generate extended header
	eh, err := cl.construct(b.Header, b.Commit, b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("making extended header: %w", err))
	}
	span.AddEvent("listener: constructed extended header",
		trace.WithAttributes(attribute.Int("square_size", eh.DAH.SquareSize())),
	)

	err = storeEDS(ctx, eh, eds, cl.store, cl.availabilityWindow, cl.archival)
	if err != nil {
		return fmt.Errorf("storing EDS: %w", err)
	}
	span.AddEvent("listener: stored square")

	syncing, err := cl.fetcher.IsSyncing(ctx)
	if err != nil {
		return fmt.Errorf("getting sync state: %w", err)
	}
	span.AddEvent("listener: fetched sync state")

	// notify network of new EDS hash only if core is already synced
	if !syncing {
		err = cl.hashBroadcaster(ctx, shrexsub.Notification{
			DataHash: eh.DataHash.Bytes(),
			Height:   eh.Height(),
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Errorw("listener: broadcasting data hash",
				"height", b.Header.Height,
				"datahash", eh.DAH.String(), "err", err)
		}
	}

	// broadcast new ExtendedHeader, but if core is still syncing, notify only local subscribers
	err = cl.headerBroadcaster.Broadcast(ctx, eh, pubsub.WithLocalPublication(syncing))
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Errorw("listener: broadcasting next header",
			"height", b.Header.Height,
			"err", err)
	}
	return nil
}
