package pruner

import (
	"context"
	"fmt"
	"time"

	header "github.com/celestiaorg/go-header"
)

// Service handles the pruning routine for the node using the
// prune Pruner.
type Service struct {
	pruner Pruner
	window AvailabilityWindow

	getter header.Store

	checkpoint checkpoint

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
			err := s.pruner.Prune(s.ctx)
			if err != nil {
				// TODO @renaynay: record + report errors properly
			}
		}
	}
}
