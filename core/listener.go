package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-app/v9/pkg/da"
	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

// Fetcher abstracts the core block source consumed by the Listener. It is
// satisfied both by a single *BlockFetcher and by a *MultiSource, which fans
// several core endpoints into one new-block stream for resilience.
type Fetcher interface {
	// Verify checks the source is on the expected network before any blocks are
	// consumed. The expected chain ID must be set — a node must know its network,
	// so an empty one is an error. A MultiSource may prune sources on the wrong
	// network here; it errors if none remain usable.
	Verify(ctx context.Context, expected string) error
	// SubscribeNewBlockEvent returns a channel of newly produced blocks.
	SubscribeNewBlockEvent(ctx context.Context) (chan SignedBlock, error)
	// ChainID returns the network/chain ID of the source.
	ChainID(ctx context.Context) (string, error)
	// IsSyncing reports whether the source is still catching up to the head.
	IsSyncing(ctx context.Context) (bool, error)
}

var (
	_ Fetcher = (*BlockFetcher)(nil)
	_ Fetcher = (*MultiSource)(nil)
)

// Listener is responsible for listening to Core for
// new block events and converting new Core blocks into
// the main data structure used in the Celestia DA network:
// `ExtendedHeader`. After digesting the Core block, extending
// it, and generating the `ExtendedHeader`, the Listener
// broadcasts the new `ExtendedHeader` to the header-sub gossipsub
// network.
type Listener struct {
	fetcher Fetcher

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
	fetcher Fetcher,
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
func (cl *Listener) Start(ctx context.Context) error {
	if cl.cancel != nil {
		return fmt.Errorf("listener: already started")
	}

	if err := cl.fetcher.Verify(ctx, cl.chainID); err != nil {
		return err
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

	// Multiple core sources (see MultiSource) fan the same height in N times.
	// Skip a height already in the store. Keying on the store (not an in-memory
	// watermark) means out-of-order blocks from a lagging source are still
	// processed, and any failure below returns without storing so another
	// source's copy retries the height. Historic blocks outside the
	// availability window are intentionally not stored, so their duplicates are
	// reprocessed per source; redundant header broadcasts are deduped by
	// gossipsub message IDs.
	if has, hasErr := cl.store.HasByHeight(ctx, uint64(b.Header.Height)); hasErr == nil && has {
		log.Debugw("listener: skipping already-processed height", "height", b.Header.Height)
		return nil
	}

	// chainID is checked only after the dedup gate, so a wrong-chain block
	// reaches it only when this source won the race for a height no other
	// source has stored yet. A duplicate from a slow misconfigured endpoint is
	// deduped away above and never panics; we crash only if the *fastest* source
	// for some height is on the wrong network — a critical misconfiguration we
	// must not silently ingest.
	if cl.chainID != "" && b.Header.ChainID != cl.chainID {
		panic(fmt.Sprintf("listener: received block with unexpected chain ID: expected %s,"+
			" received %s. blockHeight: %d blockHash: %x.",
			cl.chainID, b.Header.ChainID, b.Header.Height, b.Header.Hash()),
		)
	}

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
	bcastStart := time.Now()
	err = cl.headerBroadcaster.Broadcast(ctx, eh, pubsub.WithLocalPublication(syncing))
	cl.metrics.headerPublished(ctx, time.Since(bcastStart), syncing, err)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Errorw("listener: broadcasting next header",
			"height", b.Header.Height,
			"err", err)
	}

	cl.metrics.blockProcessed(ctx, b.Header.Time)
	return nil
}
