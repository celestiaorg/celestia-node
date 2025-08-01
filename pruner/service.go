package pruner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	contextds "github.com/ipfs/go-datastore/context"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

var log = logging.Logger("pruner/service")

// Service handles running the pruning cycle for the node.
type Service struct {
	pruner Pruner
	hstore libhead.Store[*header.ExtendedHeader]
	ds     datastore.Batching

	window    time.Duration
	blockTime time.Duration
	params    Params

	// pruneMu ensures only a single pruning operation is in progress
	pruneMu    sync.Mutex
	checkpoint *checkpoint

	metrics *metrics

	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan struct{}
}

func NewService(
	p Pruner,
	window time.Duration,
	hstore libhead.Store[*header.ExtendedHeader],
	ds datastore.Datastore,
	blockTime time.Duration,
	opts ...Option,
) (*Service, error) {
	params := DefaultParams()
	for _, opt := range opts {
		opt(&params)
	}

	if err := params.Validate(); err != nil {
		return nil, err
	}

	s := &Service{
		pruner:    p,
		window:    window,
		hstore:    hstore,
		ds:        namespace.Wrap(ds, storePrefix),
		blockTime: blockTime,
		doneCh:    make(chan struct{}),
		params:    params,
	}

	// ensure we set delete handler before all the services start
	hstore.OnDelete(s.pruneOnHeaderDelete)
	return s, nil
}

// Start loads the pruner's last pruned height (1 if pruner is freshly
// initialized) and runs the prune loop, pruning any blocks older than
// the given availability window.
func (s *Service) Start(ctx context.Context) error {
	err := s.loadCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("pruner start: loading checkpoint %w", err)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.run()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.cancel()

	s.metrics.close()

	select {
	case <-s.doneCh:
	case <-ctx.Done():
		return fmt.Errorf("pruner unable to exit within context deadline")
	}

	err := storeCheckpoint(ctx, s.ds, s.checkpoint)
	if err != nil {
		return fmt.Errorf("pruner: saving checkpoint on stop: %w", err)
	}

	return nil
}

// TODO: Exposed but accesses checkpoint without synchronization
func (s *Service) LastPruned(ctx context.Context) (uint64, error) {
	err := s.loadCheckpoint(ctx)
	if err != nil {
		return 0, err
	}
	return s.checkpoint.LastPrunedHeight, nil
}

// TODO: Exposed but accesses checkpoint without synchronization
func (s *Service) ResetCheckpoint(ctx context.Context) error {
	return s.resetCheckpoint(ctx)
}

// run prunes blocks older than the availability wiindow periodically until the
// pruner service is stopped.
func (s *Service) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.params.pruneCycle)
	defer ticker.Stop()

	for {
		s.prune(s.ctx)
		// pruning may take a while beyond ticker's time
		// and this ensures we don't do idle spins right after the pruning
		// and ensures there is always pruneCycle period between each run
		ticker.Reset(s.params.pruneCycle)

		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) prune(ctx context.Context) {
	s.pruneMu.Lock()
	defer s.pruneMu.Unlock()

	lastPrunedHeader, err := s.lastPruned(ctx)
	if err != nil {
		log.Errorw("getting last pruned header", "height", s.checkpoint.LastPrunedHeight, "err", err)
		return
	}

	// prioritize retrying previously-failed headers
	s.retryFailed(s.ctx)

	now := time.Now()
	successful, failed := 0, 0
	log.Debug("pruning round start")
	defer func() {
		log.Infow("pruning round finished", "done", successful, "failed", failed, "took", time.Since(now))
	}()

	batch, err := s.ds.Batch(ctx)
	if err != nil {
		return
	}
	ctx = contextds.WithWrite(ctx, batch)
	defer func() {
		if err := batch.Commit(ctx); err != nil {
			log.Errorw("failed to commit batch", "err", err)
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		headers, err := s.findPruneableHeaders(ctx, lastPrunedHeader)
		if err != nil || len(headers) == 0 {
			return
		}

		failedSet := make(map[uint64]struct{})

		log.Debugw("pruning headers", "from", headers[0].Height(), "to",
			headers[len(headers)-1].Height())

		for _, eh := range headers {
			pruneCtx, cancel := context.WithTimeout(ctx, time.Second*5)

			err = s.pruner.Prune(pruneCtx, eh)
			if err != nil {
				log.Errorw("failed to prune block", "height", eh.Height(), "err", err)
				failedSet[eh.Height()] = struct{}{}
				failed++
			} else {
				lastPrunedHeader = eh
				successful++
			}

			s.metrics.observePrune(pruneCtx, err != nil)
			cancel()
		}

		err = s.updateCheckpoint(s.ctx, lastPrunedHeader.Height(), failedSet)
		if err != nil {
			log.Errorw("failed to update checkpoint", "err", err)
			return
		}

		if len(headers) < maxHeadersPerLoop {
			// we've pruned all the blocks we can
			return
		}
	}
}

func (s *Service) retryFailed(ctx context.Context) {
	if len(s.checkpoint.FailedHeaders) == 0 {
		return
	}
	log.Debugw("retrying failed headers", "amount", len(s.checkpoint.FailedHeaders))

	for failed := range s.checkpoint.FailedHeaders {
		h, err := s.hstore.GetByHeight(ctx, failed)
		if err != nil {
			log.Errorw("failed to load header from failed map", "height", failed, "err", err)
			continue
		}
		err = s.pruner.Prune(ctx, h)
		if err != nil {
			log.Errorw("failed to prune block from failed map", "height", failed, "err", err)
			continue
		}
		delete(s.checkpoint.FailedHeaders, failed)
	}
}

// pruneOnHeaderDelete is called by the header Syncer whenever it prunes an old header.
// This guarantees that respective block data is always pruned on the Pruner side.
//
// There is a possible race between Syncer and Pruner, however, it is gracefully resolved.
// * If Syncer prunes a header first - pruneOnHeaderDelete gets called and data is deleted.
//   - Pruner then ignores the header based on updated checkpoint state
//
// * If Pruner prunes a header first - Syncer only removes the header
//   - pruneOnHeaderDelete ignores the header based on updated checkpoint state
func (s *Service) pruneOnHeaderDelete(ctx context.Context, height uint64) error {
	s.pruneMu.Lock()
	s.loadCheckpoint(ctx)
	if _, ok := s.checkpoint.FailedHeaders[height]; ok {
		log.Warnw("Deleted header for a height previously failed to be pruned", "height", height)
		log.Warn("Stored data for the height may never be pruned unless full resync!")
		// TODO(@Wondertan): Do we wanna give here an additional retry before removing?
		delete(s.checkpoint.FailedHeaders, height)
	}
	if height <= s.checkpoint.LastPrunedHeight {
		s.pruneMu.Unlock()
		return nil
	}
	s.pruneMu.Unlock()

	eh, err := s.hstore.GetByHeight(ctx, height)
	if err != nil {
		return fmt.Errorf("getting header %d to prune data with: %w", height, err)
	}

	err = s.pruner.Prune(ctx, eh)
	s.metrics.observePrune(ctx, err != nil)
	if err != nil {
		return fmt.Errorf("pruning height %d: %w", height, err)
	}

	s.pruneMu.Lock()
	defer s.pruneMu.Unlock()
	if height <= s.checkpoint.LastPrunedHeight {
		return nil
	}
	s.checkpoint.LastPrunedHeight = height
	// no need to persist here, as it will done in pruning routine or Stop
	log.Debugw("data pruned on header delete", "height", height)
	return nil
}
