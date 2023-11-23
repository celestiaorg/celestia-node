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
	return nil
}

func (p *Pruner) IsWithinAvailabilityWindow(_ *header.ExtendedHeader, _ time.Duration) bool {
	// TODO @renaynay: Implement in a later PR
	return true
}
