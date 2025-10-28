package byzantine

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"

	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

// ErrByzantine is a thrown when recovered data square is not correct
// (merkle proofs do not match parity erasure-coding data).
//
// It is converted from rsmt2d.ByzantineRow/Col +
// Merkle Proof for each share.
type ErrByzantine struct {
	Index  uint32
	Shares []*ShareWithProof
	Axis   rsmt2d.Axis
}

func (e *ErrByzantine) Error() string {
	return fmt.Sprintf("byzantine error(Axis:%v, Index:%v)", e.Axis, e.Index)
}

// NewErrByzantine creates new ErrByzantine from rsmt2d error.
// If error happens during proof collection, it terminates the process with os.Exit(1).
func NewErrByzantine(
	ctx context.Context,
	bStore blockstore.Blockstore,
	roots *share.AxisRoots,
	errByz *rsmt2d.ErrByzantineData,
) error {
	sharesWithProof := make([]*ShareWithProof, len(errByz.Shares))
	bGetter := ipld.NewBlockservice(bStore, nil)
	var count int
	for index, shr := range errByz.Shares {
		if len(shr) == 0 {
			continue
		}
		sh, err := libshare.NewShare(shr)
		if err != nil {
			log.Warn("failed to create share", "index", index, "err", err)
			continue
		}
		swp, err := GetShareWithProof(ctx, bGetter, roots, *sh, errByz.Axis, int(errByz.Index), index)
		if err != nil {
			log.Warn("requesting proof failed",
				"errByz", errByz,
				"shareIndex", index,
				"err", err)
			continue
		}

		sharesWithProof[index] = swp
		// it is enough to collect half of the shares to construct the befp
		if count++; count >= len(roots.RowRoots)/2 {
			break
		}
	}

	if count < len(roots.RowRoots)/2 {
		return fmt.Errorf("failed to collect proof")
	}

	return &ErrByzantine{
		Index:  uint32(errByz.Index),
		Shares: sharesWithProof,
		Axis:   errByz.Axis,
	}
}
