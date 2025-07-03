package pruner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
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
	ds     datastore.Datastore

	window    time.Duration
	blockTime time.Duration
	params    Params

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
		pruner:     p,
		window:     window,
		hstore:     hstore,
		checkpoint: &checkpoint{FailedHeaders: map[uint64]struct{}{}},
		ds:         namespace.Wrap(ds, storePrefix),
		blockTime:  blockTime,
		doneCh:     make(chan struct{}),
		params:     params,
	}

	// ensure we set delete handler before all the services start
	hstore.OnDelete(s.onHeadersPrune)
	return s, nil
}

// Start loads the pruner's last pruned height (1 if pruner is freshly
// initialized) and runs the prune loop, pruning any blocks older than
// the given availability window.
func (s *Service) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	err := s.loadCheckpoint(s.ctx)
	if err != nil {
		return fmt.Errorf("pruner start: loading checkpoint %w", err)
	}
	log.Debugw("loaded checkpoint", "lastPruned", s.checkpoint.LastPrunedHeight)

	go s.run()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.cancel()

	s.metrics.close()

	select {
	case <-s.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("pruner unable to exit within context deadline")
	}
}

func (s *Service) LastPruned(ctx context.Context) (uint64, error) {
	err := s.loadCheckpoint(ctx)
	if err != nil {
		return 0, err
	}
	return s.checkpoint.LastPrunedHeight, nil
}

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
	log.Debug("pruning round start")
	defer func() {
		log.Debugw("pruning round finished", "took", time.Since(now))
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

		failed := make(map[uint64]struct{})

		log.Debugw("pruning headers", "from", headers[0].Height(), "to",
			headers[len(headers)-1].Height())

		for _, eh := range headers {
			pruneCtx, cancel := context.WithTimeout(ctx, time.Second*5)

			err = s.pruner.Prune(pruneCtx, eh)
			if err != nil {
				log.Errorw("failed to prune block", "height", eh.Height(), "err", err)
				failed[eh.Height()] = struct{}{}
			} else {
				lastPrunedHeader = eh
			}

			s.metrics.observePrune(pruneCtx, err != nil)
			cancel()
		}

		err = s.updateCheckpoint(s.ctx, lastPrunedHeader.Height(), failed)
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

// onHeadersPrune is called by the header Syncer whenever it prunes old headers.
// This guarantees that respective block data for those headers is always pruned on the Pruner side.
//
// There is a possible race between Syncer and Pruner, however, it is gracefully resolved.
// * If Syncer prunes a range of headers first - onHeadersPrune gets called and data is deleted.
//   - Pruner then ignores the range based on updated checkpoint state
//
// * If Pruner prunes a range first - Syncer only removes headers
//   - onHeadersPrune ignores the range based on updated checkpoint state
func (s *Service) onHeadersPrune(ctx context.Context, headers []*header.ExtendedHeader) error {
	s.pruneMu.Lock()
	defer s.pruneMu.Unlock()

	log.Debugw("pruning headers", "from", headers[0].Height(), "to",
		headers[len(headers)-1].Height())

	var lastPrunedHeader *header.ExtendedHeader
	failed := make(map[uint64]struct{})
	for _, eh := range headers {
		if _, ok := s.checkpoint.FailedHeaders[eh.Height()]; ok {
			log.Warnw("Deleted header for a height previously failed to be pruned", "height", eh.Height())
			log.Warn("Stored data for the height may never be pruned unless full resync!")
			// TODO(@Wondertan): Do we wanna give here an additional retry before removing?
			delete(s.checkpoint.FailedHeaders, eh.Height())
		}
		if eh.Height() <= s.checkpoint.LastPrunedHeight {
			continue
		}

		// TODO(@Wondertan): make it a configurable value
		pruneCtx, cancel := context.WithTimeout(ctx, time.Second*5)

		err := s.pruner.Prune(pruneCtx, eh)
		if err != nil {
			log.Errorw("failed to prune block", "height", eh.Height(), "err", err)
			failed[eh.Height()] = struct{}{}
		} else {
			lastPrunedHeader = eh
		}

		s.metrics.observePrune(pruneCtx, err != nil)
		cancel()
	}

	err := s.updateCheckpoint(s.ctx, lastPrunedHeader.Height(), failed)
	if err != nil {
		log.Errorw("failed to update checkpoint", "err", err)
	}

	return nil
}
