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
	"github.com/celestiaorg/celestia-node/share/availability"
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
	// SubscribeNewBlockEvent returns a channel of new-block events. Each event
	// carries the height and which source announced it; the block is fetched on
	// demand via GetSignedBlockFrom, so a MultiSource downloads each block once
	// instead of once per source.
	SubscribeNewBlockEvent(ctx context.Context) (chan BlockEvent, error)
	// GetSignedBlockFrom fetches the full signed block for the event from the
	// source that announced it — the peer fastest to notify this height. It does
	// NOT fall back to other sources on failure: an error is just an error.
	// Resilience is a property of the fan-in — another source announces the same
	// height, and the Listener retries the fetch from it (the failed attempt
	// stored nothing, so the duplicate is a store-miss, not a skip).
	GetSignedBlockFrom(ctx context.Context, ev BlockEvent) (*SignedBlock, error)
	// ChainID returns the network/chain ID of the source.
	ChainID(ctx context.Context) (string, error)
	// IsSyncingFrom reports whether the source that announced the event is still
	// catching up to the head. Sync state is per-source: the answer must come
	// from the same peer that served the block, since another source being
	// caught up says nothing about whether THIS block is a fresh head or a
	// replay of an old height from a peer mid-blocksync. Like
	// GetSignedBlockFrom, it does not fall back to other sources on failure.
	IsSyncingFrom(ctx context.Context, ev BlockEvent) (bool, error)
}

// blockSource is a single core endpoint: the leaf a MultiSource fans over. It
// is deliberately narrower than Fetcher — no GetSignedBlockFrom, since
// resolving an announced event to a source is the MultiSource's responsibility,
// not a single endpoint's.
type blockSource interface {
	SubscribeNewBlockEvent(ctx context.Context) (chan BlockEvent, error)
	GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error)
	ChainID(ctx context.Context) (string, error)
	IsSyncing(ctx context.Context) (bool, error)
}

// blockFetchTimeout bounds a single block fetch made while handling a height
// announced by a source.
// TODO(@vgonkivs): make timeout configurable
const blockFetchTimeout = 10 * time.Second

var (
	_ Fetcher     = (*BlockFetcher)(nil)
	_ Fetcher     = (*MultiSource)(nil)
	_ blockSource = (*BlockFetcher)(nil)
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
func (cl *Listener) listen(ctx context.Context, sub <-chan BlockEvent) {
	defer close(cl.closed)
	defer log.Info("listener: listening stopped")
	timeout := time.NewTimer(cl.listenerTimeout)
	defer timeout.Stop()
	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				log.Error("underlying subscription was closed")
				return
			}

			log.Debugw("listener: new block height from core", "height", ev.Height)

			err := cl.handleNewBlockEvent(ctx, ev)
			if err != nil {
				log.Errorw("listener: handling new block event",
					"height", ev.Height,
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

// handleNewBlockEvent processes a height announced by a core source. Multiple
// sources (see MultiSource) announce the same height N times; the store dedup
// gate here ensures the block is downloaded and processed exactly once. Because
// events are handled sequentially in listen(), a duplicate height is read only
// after the first copy has been stored, so it is skipped before any redundant
// download. Keying on the store (not an in-memory watermark) means out-of-order
// heights from a lagging source are still processed, and any failure below
// returns without storing so another source's announcement retries the height.
// Historic blocks outside the availability window are dropped right after the
// fetch — never stored — so their duplicates are re-downloaded per source (the
// event carries only the height; the block time is unknown until fetched), but
// skip the expensive processing below; redundant header broadcasts are deduped
// by gossipsub message IDs.
func (cl *Listener) handleNewBlockEvent(ctx context.Context, ev BlockEvent) error {
	has, err := cl.store.HasByHeight(ctx, uint64(ev.Height))
	if err != nil {
		log.Errorw("listener: error checking block height in store", "height", ev.Height, "err", err)
		cl.metrics.blockEvent(ctx, ev.addr, "store_error")
		return nil
	}
	if has {
		log.Debugw("listener: skipping already-processed height", "height", ev.Height)
		cl.metrics.blockEvent(ctx, ev.addr, "duplicate")
		return nil
	}

	// Fetch the full block on demand from the source that announced it first —
	// the fastest peer to notify this height — so a MultiSource downloads it
	// once rather than once per source.
	fetchCtx, cancel := context.WithTimeout(ctx, blockFetchTimeout)
	b, err := cl.fetcher.GetSignedBlockFrom(fetchCtx, ev)
	cancel()
	if err != nil {
		cl.metrics.blockEvent(ctx, ev.addr, "fetch_error")
		return fmt.Errorf("fetching signed block at height %d: %w", ev.Height, err)
	}

	// A source replaying history during catch-up (blocksync) announces heights
	// the pruned bridge will never keep. Drop them as soon as the block time is
	// known — before the sync-status query and, crucially, before the
	// erasure-coding pass in handleNewSignedBlock — or a from-scratch source
	// would cost a full EDS construction per replayed height just for storeEDS
	// to throw the result away. Mirrors the storeEDS predicate exactly.
	if !cl.archival && !availability.IsWithinWindow(b.Header.Time, cl.availabilityWindow) {
		log.Debugw("listener: skipping historic block outside availability window",
			"height", ev.Height, "source", ev.addr)
		cl.metrics.blockEvent(ctx, ev.addr, "historic")
		return nil
	}

	// Ask the announcing source — and only it — whether it is still catching
	// up: that decides if this block is a fresh head (broadcast to the network)
	// or a catch-up replay (publish locally). Queried before the block is
	// stored, so a failure here leaves the height unstored and the duplicate
	// announcement from another source retries it whole.
	syncCtx, cancel := context.WithTimeout(ctx, blockFetchTimeout)
	syncing, err := cl.fetcher.IsSyncingFrom(syncCtx, ev)
	cancel()
	if err != nil {
		cl.metrics.blockEvent(ctx, ev.addr, "sync_error")
		return fmt.Errorf("getting sync state for height %d: %w", ev.Height, err)
	}

	if err := cl.handleNewSignedBlock(ctx, ev, b, syncing); err != nil {
		cl.metrics.blockEvent(ctx, ev.addr, "process_error")
		return err
	}
	cl.metrics.blockEvent(ctx, ev.addr, "processed")
	return nil
}

func (cl *Listener) handleNewSignedBlock(ctx context.Context, ev BlockEvent, b *SignedBlock, syncing bool) error {
	var err error

	ctx, span := tracer.Start(ctx, "listener/handleNewSignedBlock")
	defer func() {
		utils.SetStatusAndEnd(span, err)
	}()
	span.SetAttributes(
		attribute.Int64("height", b.Header.Height),
	)

	// chainID is checked only after the dedup gate (in handleNewBlockEvent), so a
	// wrong-chain block reaches it only when this source won the race for a height
	// no other source has stored yet. A duplicate from a slow misconfigured
	// endpoint is deduped away before fetching and never panics; we crash only if
	// the *fastest* source for some height is on the wrong network — a critical
	// misconfiguration we must not silently ingest. The source address is named so
	// the operator knows which configured endpoint to remove.
	if cl.chainID != "" && b.Header.ChainID != cl.chainID {
		panic(fmt.Sprintf("listener: source %q served a block on the wrong chain: expected %s,"+
			" received %s. blockHeight: %d blockHash: %x.",
			ev.addr, cl.chainID, b.Header.ChainID, b.Header.Height, b.Header.Hash()),
		)
	}

	eds, err := da.ConstructEDS(b.Data.Txs.ToSliceOfBytes(), b.Header.Version.App, -1)
	if err != nil {
		return fmt.Errorf("extending block data: %w", err)
	}

	// generate extended header
	eh, err := cl.construct(b.Header, b.Commit, b.ValidatorSet, eds)
	if err != nil {
		panic(fmt.Errorf("listener: source %q served a block with inconsistent data at height %d: "+
			"making extended header: %w", ev.addr, b.Header.Height, err))
	}
	span.AddEvent("listener: constructed extended header",
		trace.WithAttributes(attribute.Int("square_size", eh.DAH.SquareSize())),
	)

	err = storeEDS(ctx, eh, eds, cl.store, cl.availabilityWindow, cl.archival)
	if err != nil {
		return fmt.Errorf("storing EDS: %w", err)
	}
	span.AddEvent("listener: stored square")

	// notify network of new EDS hash only if the announcing source is already
	// synced (sync state was fetched from it in handleNewBlockEvent)
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
