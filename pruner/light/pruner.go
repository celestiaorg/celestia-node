package light

import (
	"context"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

type Pruner struct {
	bserv blockservice.BlockService
	ds    datastore.Datastore
}

func NewPruner(bstore blockstore.Blockstore, ds datastore.Batching) pruner.Pruner {
	return &Pruner{bserv: ipld.NewBlockservice(bstore, nil), ds: ds}
}

func (p *Pruner) Prune(ctx context.Context, h *header.ExtendedHeader) error {
	dah := h.DAH
	if share.DataHash(dah.Hash()).IsEmptyRoot() {
		return nil
	}

	var roots [][]byte
	roots = append(roots, h.DAH.RowRoots...)
	roots = append(roots, h.DAH.ColumnRoots...)
	for _, root := range roots {
		cid := ipld.MustCidFromNamespacedSha256(root)
		if err := ipld.DeleteNode(ctx, p.bserv, cid); err != nil {
			return err
		}
	}

	return p.ds.Delete(ctx, rootKey(dah))
}

func rootKey(root *share.Root) datastore.Key {
	return datastore.NewKey(root.String())
}
