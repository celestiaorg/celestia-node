package full

import (
	"context"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds"
)

type Pruner struct {
	store *eds.Store
}

func NewPruner(store *eds.Store) *Pruner {
	return &Pruner{
		store: store,
	}
}

func (p *Pruner) Prune(ctx context.Context, headers ...header.ExtendedHeader) error {
	for _, h := range headers {
		// TODO: Change Prune signature to return slice of failed headers
		if err := p.store.Remove(ctx, h.DAH.Hash()); err != nil {
			return err
		}
	}
	return nil
}
