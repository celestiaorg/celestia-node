package retriever

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/ipfs/go-blockservice"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/rsmt2d"
)

// ErrByzantine is a thrown when recovered data square is not correct
// (merkle proofs do not match parity erasure-coding data).
//
// It is converted from rsmt2d.ByzantineRow/Col +
// Merkle Proof for each share.
type ErrByzantine struct {
	Index  uint32
	Shares []*share.ShareWithProof
	Axis   rsmt2d.Axis
}

func (e *ErrByzantine) Error() string {
	return fmt.Sprintf("byzantine error(Axis:%v, Index:%v)", e.Axis, e.Index)
}

// NewErrByzantine creates new ErrByzantine from rsmt2d error.
// If error happens during proof collection, it terminates the process with os.Exit(1).
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
	sharesWithProof, err := share.GetProofsForShares(
		ctx,
		bGetter,
		ipld.MustCidFromNamespacedSha256(root),
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
		Index:  uint32(errByz.Index),
		Shares: sharesWithProof,
		Axis:   errByz.Axis,
	}
}
