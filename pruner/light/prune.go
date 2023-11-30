package light

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

type Pruner struct{}

func NewPruner() *Pruner {
	return &Pruner{}
}

func (p *Pruner) Prune(context.Context, ...*header.ExtendedHeader) error {
	// TODO @renaynay: Implement once IPLDv2 is ready
	return nil
}

func (p *Pruner) IsWithinAvailabilityWindow(eh *header.ExtendedHeader, window time.Duration) bool {
	return time.Since(eh.Time()) <= window
}
