package full

import (
	"context"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds"
)

var log = logging.Logger("pruner/full")

type Pruner struct {
	store *eds.Store
}

func NewPruner(store *eds.Store) *Pruner {
	return &Pruner{
		store: store,
	}
}

func (p *Pruner) Prune(ctx context.Context, headers ...*header.ExtendedHeader) error {
	for _, h := range headers {
		log.Debugf("pruning header %s", h.DAH.Hash())
		if err := p.store.Remove(ctx, h.DAH.Hash()); err != nil {
			return err
		}
	}
	return nil
}
