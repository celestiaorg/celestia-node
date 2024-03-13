package pruner

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"

	hdr "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

var log = logging.Logger("pruner/service")

// Service handles running the pruning cycle for the node.
type Service struct {
	pruner Pruner
	window AvailabilityWindow

	getter hdr.Getter[*header.ExtendedHeader]

	ds         datastore.Datastore
	checkpoint *checkpoint

	numBlocksInWindow uint64

	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan struct{}

	params  Params
	metrics *metrics
}

func NewService(
	p Pruner,
	window AvailabilityWindow,
	getter hdr.Getter[*header.ExtendedHeader],
	ds datastore.Datastore,
	blockTime time.Duration,
	opts ...Option,
) *Service {
	params := DefaultParams()
	for _, opt := range opts {
		opt(&params)
	}

	numBlocksInWindow := uint64(time.Duration(window) / blockTime)

	return &Service{
		pruner:            p,
		window:            window,
		getter:            getter,
		checkpoint:        &checkpoint{FailedHeaders: map[uint64]struct{}{}},
		ds:                namespace.Wrap(ds, storePrefix),
		numBlocksInWindow: numBlocksInWindow,
		doneCh:            make(chan struct{}),
		params:            params,
	}
}

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

	select {
	case <-s.doneCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("pruner unable to exit within context deadline")
	}
}

func (s *Service) run() {
	defer close(s.doneCh)

	if s.params.gcCycle == time.Duration(0) {
		// Service is disabled, exit
		return
	}

	ticker := time.NewTicker(s.params.gcCycle)
	defer ticker.Stop()

	lastPrunedHeader, err := s.lastPruned(s.ctx)
	if err != nil {
		log.Errorw("failed to get last pruned header", "height", s.checkpoint.LastPrunedHeight,
			"err", err)
		log.Warn("exiting pruner service!")
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			lastPrunedHeader = s.prune(s.ctx, lastPrunedHeader)
		}
	}
}

func (s *Service) prune(
	ctx context.Context,
	lastPrunedHeader *header.ExtendedHeader,
) *header.ExtendedHeader {
	// prioritize retrying previously-failed headers
	s.retryFailed(s.ctx)

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
			pruneCtx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*5))

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

		if uint64(len(headers)) < maxHeadersPerLoop {
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
