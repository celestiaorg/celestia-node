package pruner

import (
	"context"
)

// Service handles the pruning routine for the node using the
// prune Pruner.
type Service struct {
	pruner Pruner
}

func NewService(p Pruner) *Service {
	return &Service{
		pruner: p,
	}
}

func (s *Service) Start(context.Context) error {
	return nil
}

func (s *Service) Stop(context.Context) error {
	return nil
}
