package ipld

import (
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/rsmt2d"
)

// ErrByzantine is a thrown when recovered data square is not correct
// (merkle proofs do not match parity erasure-coding data).
//
// It is converted from rsmt2d.ByzantineRow/Col +
// Merkle Proof for each share.
type ErrByzantine struct {
	Index  uint8
	Shares []*ShareWithProof
	// TODO(@vgokivs): Change to enum type and rename to Axis after
	// updating rsmt2d
	IsRow bool
}

func (e *ErrByzantine) Error() string {
	return fmt.Sprintf("byzantine error. isRow:%v, Index:%v", e.IsRow, e.Index)
}

// NewErrByzantine creates new ErrByzantine from rsmt2d error.
// If error happens during proof collection, it terminates the process with os.Exit(1).
// TODO(@Wondertan): Migrate to ErrByzantineData in the newest rsmt2d
func NewErrByzantine(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	dah *da.DataAvailabilityHeader,
	errByz *rsmt2d.ErrByzantineData,
) *ErrByzantine {
	root := [][][]byte{
		dah.RowsRoots,
		dah.ColumnRoots,
	}[errByz.Axis][errByz.Index]
	sharesWithProof, err := GetProofsForShares(
		ctx,
		bGetter,
		plugin.MustCidFromNamespacedSha256(root),
		errByz.Shares,
	)
	if err != nil {
		// Fatal as rsmt2d proved that error is byzantine,
		// but we cannot properly collect the proof,
		// so verification will fail and thus services won't be stopped
		// while we still have to stop them.
		// TODO(@Wondertan): Find a better way to handle
		log.Fatalw("getting proof for ErrByzantine", "err", err)
	}

	return &ErrByzantine{
		Index:  uint8(errByz.Index),
		Shares: sharesWithProof,
		IsRow:  errByz.Axis == rsmt2d.Row,
	}
}
