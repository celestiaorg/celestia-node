package byzantine

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockservice"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-app/pkg/da"
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
	// changing the order to collect proofs against an orthogonal axis
	axisType, roots := rsmt2d.Row, dah.RowRoots
	if errByz.Axis == rsmt2d.Row {
		axisType, roots = rsmt2d.Col, dah.ColumnRoots
	}

	sharesWithProof := make([]*share.ShareWithProof, len(errByz.Shares))
	sharesAmount := 0

	errGr, ctx := errgroup.WithContext(ctx)
	for index, share := range errByz.Shares {
		// skip further shares if we already requested half of them, which is enough to recompute the row
		// or col
		if sharesAmount == len(dah.RowRoots)/2 {
			break
		}

		if share == nil {
			continue
		}
		sharesAmount++

		index := index
		errGr.Go(func() error {
			sh, err := ipld.GetShareWithProof(
				ctx, bGetter,
				roots[index],
				int(errByz.Index), len(errByz.Shares),
				axisType,
			)
			sharesWithProof[index] = sh
			return err
		})
	}

	if err := errGr.Wait(); err != nil {
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
