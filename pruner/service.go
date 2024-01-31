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

/*
	TODO:
		* failed headers retry loop / jobs
*/

// Service handles the pruning routine for the node using the
// prune Pruner.
type Service struct {
	pruner Pruner
	window AvailabilityWindow

	getter hdr.Getter[*header.ExtendedHeader]

	ds         datastore.Datastore
	checkpoint *checkpoint

	// TODO @renaynay: how would this impact a node that enables pruning after being an archival node?
	//  e.g. Node has already 600k+ blocks stored, how long will it take to clean up all blocks outside
	//  of pruning window?
	maxPruneablePerGC uint64
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
		checkpoint:        &checkpoint{FailedHeaders: map[uint64]string{}},
		ds:                namespace.Wrap(ds, storePrefix),
		numBlocksInWindow: numBlocksInWindow,
		maxPruneablePerGC: params.maxPruneablePerGC,
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
	log.Debugw("loaded checkpoint", "lastPruned", s.lastPruned().Height())

	go s.prune()
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

func (s *Service) prune() {
	if s.params.gcCycle == time.Duration(0) {
		// Service is disabled, exit
		close(s.doneCh)
		return
	}

	ticker := time.NewTicker(s.params.gcCycle)
	defer ticker.Stop()

	// prioritize retrying failed headers
	s.retryFailed(s.ctx)

	lastPrunedHeader := s.lastPruned()

	for {
		select {
		case <-s.ctx.Done():
			close(s.doneCh)
			return
		case <-ticker.C:
			headers, err := s.findPruneableHeaders(s.ctx)
			if err != nil || len(headers) == 0 {
				// TODO @renaynay: record errors properly
				log.Errorw("failed to find prune-able blocks", "error", err)
				continue
			}

			failed := make(map[uint64]error)

			log.Debugw("pruning headers", "from", headers[0].Height(), "to",
				headers[len(headers)-1].Height())

			for _, eh := range headers {
				pruneCtx, cancel := context.WithDeadline(s.ctx, time.Now().Add(time.Second))

				err = s.pruner.Prune(pruneCtx, eh)
				if err != nil {
					log.Errorw("failed to prune block", "height", eh.Height(), "err", err)
					failed[eh.Height()] = err
				} else {
					// TODO @renaynay: make prettier
					lastPrunedHeader = eh
				}

				s.metrics.observePrune(pruneCtx, err != nil)
				cancel()
			}

			err = s.updateCheckpoint(s.ctx, lastPrunedHeader, failed)
			if err != nil {
				log.Errorw("failed to update checkpoint", "err", err)
				continue
			}

			log.Debugw("retrying failed headers", "amount", len(s.checkpoint.FailedHeaders))
			s.retryFailed(s.ctx) // TODO @renaynay: persist the results of this to disk
		}
	}
}

func (s *Service) retryFailed(ctx context.Context) {
	for failed := range s.checkpoint.FailedHeaders {
		h, err := s.getter.GetByHeight(ctx, failed)
		if err != nil {
			log.Errorw("failed to load header from failed map", "height", failed, "err", err)
			s.checkpoint.FailedHeaders[failed] = err.Error()
			continue
		}
		err = s.pruner.Prune(ctx, h)
		if err != nil {
			log.Errorw("failed to prune block from failed map", "height", failed, "err", err)
			s.checkpoint.FailedHeaders[failed] = err.Error()
			continue
		}
		delete(s.checkpoint.FailedHeaders, failed)
	}
}
