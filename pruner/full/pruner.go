package full

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
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

func (p *Pruner) Prune(ctx context.Context, eh *header.ExtendedHeader) error {
	// short circuit on empty roots
	if eh.DAH.Equals(share.EmptyRoot()) {
		return nil
	}

	log.Debugf("pruning header %s", eh.DAH.Hash())

	err := p.store.Remove(ctx, eh.DAH.Hash())
	if err != nil && !errors.Is(err, dagstore.ErrShardUnknown) {
		return err
	}
	return nil
}
