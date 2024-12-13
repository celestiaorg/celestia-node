package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

var (
	tracer                 = otel.Tracer("core/listener")
	retrySubscriptionDelay = 5 * time.Second

	errInvalidSubscription = errors.New("invalid subscription")
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

	sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
	if err != nil {
		return err
	}
	go cl.runSubscriber(ctx, sub)
	return nil
}

// Stop stops the listener loop.
func (cl *Listener) Stop(ctx context.Context) error {
	err := cl.fetcher.UnsubscribeNewBlockEvent(ctx)
	if err != nil {
		log.Warnw("listener: unsubscribing from new block event", "err", err)
	}

	cl.cancel()
	select {
	case <-cl.closed:
		cl.cancel = nil
		cl.closed = nil
	case <-ctx.Done():
		return ctx.Err()
	}

	err = cl.metrics.Close()
	if err != nil {
		log.Warnw("listener: closing metrics", "err", err)
	}
	return nil
}

// runSubscriber runs a subscriber to receive event data of new signed blocks. It will attempt to
// resubscribe in case error happens during listening of subscription
func (cl *Listener) runSubscriber(ctx context.Context, sub <-chan types.EventDataSignedBlock) {
	defer close(cl.closed)
	for {
		err := cl.listen(ctx, sub)
		if ctx.Err() != nil {
			// listener stopped because external context was canceled
			return
		}
		if errors.Is(err, errInvalidSubscription) {
			// stop node if there is a critical issue with the block subscription
			log.Fatalf("listener: %v", err) //nolint:gocritic
		}

		log.Warnw("listener: subscriber error, resubscribing...", "err", err)
		sub = cl.resubscribe(ctx)
		if sub == nil {
			return
		}
	}
}

func (cl *Listener) resubscribe(ctx context.Context) <-chan types.EventDataSignedBlock {
	err := cl.fetcher.UnsubscribeNewBlockEvent(ctx)
	if err != nil {
		log.Warnw("listener: unsubscribe", "err", err)
	}

	ticker := time.NewTicker(retrySubscriptionDelay)
	defer ticker.Stop()
	for {
		sub, err := cl.fetcher.SubscribeNewBlockEvent(ctx)
		if err == nil {
			return sub
		}
		log.Errorw("listener: resubscribe", "err", err)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// listen kicks off a loop, listening for new block events from Core,
// generating ExtendedHeaders and broadcasting them to the header-sub
// gossipsub network.
func (cl *Listener) listen(ctx context.Context, sub <-chan types.EventDataSignedBlock) error {
	defer log.Info("listener: listening stopped")
	timeout := time.NewTimer(cl.listenerTimeout)
	defer timeout.Stop()
	for {
		select {
		case b, ok := <-sub:
			if !ok {
				return errors.New("underlying subscription was closed")
			}

			if cl.chainID != "" && b.Header.ChainID != cl.chainID {
				log.Errorf("listener: received block with unexpected chain ID: expected %s,"+
					" received %s", cl.chainID, b.Header.ChainID)
				return errInvalidSubscription
			}

			log.Debugw("listener: new block from core", "height", b.Header.Height)

			err := cl.handleNewSignedBlock(ctx, b)
			if err != nil {
				log.Errorw("listener: handling new block msg",
					"height", b.Header.Height,
					"hash", b.Header.Hash().String(),
					"err", err)
			}

			if !timeout.Stop() {
				<-timeout.C
			}
			timeout.Reset(cl.listenerTimeout)
		case <-timeout.C:
			cl.metrics.subscriptionStuck(ctx)
			return errors.New("underlying subscription is stuck")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (cl *Listener) handleNewSignedBlock(ctx context.Context, b types.EventDataSignedBlock) error {
	ctx, span := tracer.Start(ctx, "handle-new-signed-block")
	defer span.End()
	span.SetAttributes(
		attribute.Int64("height", b.Header.Height),
	)

	eds, err := extendBlock(b.Data, b.Header.Version.App)
	if err != nil {
		return fmt.Errorf("extending block data: %w", err)
	}

	// generate extended header
	eh, err := cl.construct(&b.Header, &b.Commit, &b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("making extended header: %w", err))
	}

	err = storeEDS(ctx, eh, eds, cl.store, cl.availabilityWindow, cl.archival)
	if err != nil {
		return fmt.Errorf("storing EDS: %w", err)
	}

	syncing, err := cl.fetcher.IsSyncing(ctx)
	if err != nil {
		return fmt.Errorf("getting sync state: %w", err)
	}

	// notify network of new EDS hash only if core is already synced
	if !syncing {
		err = cl.hashBroadcaster(ctx, shrexsub.Notification{
			DataHash: eh.DataHash.Bytes(),
			Height:   eh.Height(),
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Errorw("listener: broadcasting data hash",
				"height", b.Header.Height,
				"hash", b.Header.Hash(), "err", err) // TODO: hash or datahash?
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
