package light

import (
	"context"

	"github.com/ipfs/boxo/blockservice"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

type Pruner struct {
	bserv blockservice.BlockService
}

func NewPruner(bserv blockservice.BlockService) *Pruner {
	return &Pruner{bserv: bserv}
}

func (p *Pruner) Prune(ctx context.Context, h *header.ExtendedHeader) error {
	var roots [][]byte
	roots = append(roots, h.DAH.RowRoots...)
	roots = append(roots, h.DAH.ColumnRoots...)
	for _, root := range roots {
		cid := ipld.MustCidFromNamespacedSha256(root)
		if err := ipld.DeleteNode(ctx, p.bserv, cid); err != nil {
			return err
		}
	}

	return nil
}
