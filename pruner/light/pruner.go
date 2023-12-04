package light

import (
	"context"

	"github.com/celestiaorg/celestia-node/header"
)

type Pruner struct{}

func NewPruner() *Pruner {
	return &Pruner{}
}

func (p *Pruner) Prune(context.Context, ...*header.ExtendedHeader) error {
	return nil
}
