package pruner

import (
	"context"
	"fmt"
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
	window AvailabilityWindow

	getter libhead.Getter[*header.ExtendedHeader]

	ds         datastore.Datastore
	checkpoint *checkpoint

	blockTime time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan struct{}

	params  Params
	metrics *metrics
}

func NewService(
	p Pruner,
	window AvailabilityWindow,
	getter libhead.Getter[*header.ExtendedHeader],
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

	return &Service{
		pruner:     p,
		window:     window,
		getter:     getter,
		checkpoint: &checkpoint{FailedHeaders: map[uint64]struct{}{}},
		ds:         namespace.Wrap(ds, storePrefix),
		blockTime:  blockTime,
		doneCh:     make(chan struct{}),
		params:     params,
	}, nil
}

// Start loads the pruner's last pruned height (1 if pruner is freshly
// initialized) and runs the prune loop, pruning any blocks older than
// the given availability window.
func (s *Service) Start(context.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	err := s.loadCheckpoint(s.ctx)
	if err != nil {
		return err
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

// run prunes blocks older than the availability wiindow periodically until the
// pruner service is stopped.
func (s *Service) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.params.pruneCycle)
	defer ticker.Stop()

	lastPrunedHeader, err := s.lastPruned(s.ctx)
	if err != nil {
		log.Errorw("failed to get last pruned header", "height", s.checkpoint.LastPrunedHeight,
			"err", err)
		log.Warn("exiting pruner service!")

		s.cancel()
	}

	for {
		lastPrunedHeader = s.prune(s.ctx, lastPrunedHeader)
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

func (s *Service) prune(
	ctx context.Context,
	lastPrunedHeader *header.ExtendedHeader,
) *header.ExtendedHeader {
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
			return lastPrunedHeader
		default:
		}

		headers, err := s.findPruneableHeaders(ctx, lastPrunedHeader)
		if err != nil || len(headers) == 0 {
			return lastPrunedHeader
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
			return lastPrunedHeader
		}

		if len(headers) < maxHeadersPerLoop {
			// we've pruned all the blocks we can
			return lastPrunedHeader
		}
	}
}

func (s *Service) retryFailed(ctx context.Context) {
	log.Debugw("retrying failed headers", "amount", len(s.checkpoint.FailedHeaders))

	for failed := range s.checkpoint.FailedHeaders {
		h, err := s.getter.GetByHeight(ctx, failed)
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
