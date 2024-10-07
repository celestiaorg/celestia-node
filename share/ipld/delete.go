package ipld

import (
	"context"
	"errors"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"
)

// DeleteNode deletes the Node behind the CID. It also recursively deletes all other Nodes linked
// behind the Node.
func DeleteNode(ctx context.Context, bserv blockservice.BlockService, cid cid.Cid) error {
	blk, err := GetNode(ctx, bserv, cid)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			return nil
		}
		return err
	}

	for _, lnk := range blk.Links() {
		if err := DeleteNode(ctx, bserv, lnk.Cid); err != nil {
			return err
		}
	}

	return bserv.DeleteBlock(ctx, cid)
}
