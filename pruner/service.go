package pruner

import (
	"context"
	"fmt"
	"github.com/ipfs/go-datastore"
	"time"

	"github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
)

// Service handles the pruning routine for the node using the
// prune Pruner.
type Service struct {
	pruner Pruner
	window AvailabilityWindow

	getter store.Store[*header.ExtendedHeader]

	checkpoint   checkpoint
	checkpointDS datastore.Datastore

	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan struct{}

	params Params
}

func NewService(p Pruner, window AvailabilityWindow) *Service {
	return &Service{
		pruner: p,
		window: window,
	}
}

func (s *Service) Start(context.Context) error {
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
	ticker := time.NewTicker(s.params.gcCycle)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			close(s.doneCh)
			return
		case <-ticker.C:
			headers, err := s.findPruneableHeaders()
			if err != nil {
				// TODO @renaynay: record + report errors properly
				continue
			}
			// TODO @renaynay: make deadline a param ? / configurable?
			pruneCtx, cancel := context.WithDeadline(s.ctx, time.Now().Add(time.Minute))
			err = s.pruner.Prune(pruneCtx, headers...)
			cancel()
			if err != nil {
				// TODO @renaynay: record + report errors properly
				continue
			}

			err = s.updateCheckpoint(s.ctx, headers[len(headers)-1].Height())
			if err != nil {
				// TODO @renaynay: record + report errors properly
			}
		}
	}
}
