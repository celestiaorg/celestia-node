package pruner

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

// AvailabilityWindow defines an interface for pruning historical data.
type AvailabilityWindow interface {
	// IsWithinAvailabilityWindow returns true if the given header is within the
	// "availability" (sampling) window of the network.
	IsWithinAvailabilityWindow(eh *header.ExtendedHeader, window time.Duration) bool
}

// Pruner handles the pruning routine for the node using the
// prune Factory.
type Pruner struct {
	factory Factory
}

func NewPruner(factory Factory) *Pruner {
	return &Pruner{
		factory: factory,
	}
}

func (p *Pruner) Start(context.Context) error {
	return nil
}

func (p *Pruner) Stop(context.Context) error {
	return nil
}

// IsWithinAvailabilityWindow returns whether the given header is within the
// availability window for the underlying prune Factory.
func (p *Pruner) IsWithinAvailabilityWindow(eh *header.ExtendedHeader, window time.Duration) bool {
	return p.factory.IsWithinAvailabilityWindow(eh, window)
}
