package byzantine

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockservice"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share/ipld"
)

// ErrByzantine is a thrown when recovered data square is not correct
// (merkle proofs do not match parity erasure-coding data).
//
// It is converted from rsmt2d.ByzantineRow/Col +
// Merkle Proof for each share.
type ErrByzantine struct {
	Shares []*ShareWithProof
	Axis   rsmt2d.Axis
	Index  uint32
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
) error {
	// changing the order to collect proofs against an orthogonal axis
	roots := [][][]byte{
		dah.ColumnRoots,
		dah.RowRoots,
	}[errByz.Axis]

	sharesWithProof := make([]*ShareWithProof, len(errByz.Shares))

	type result struct {
		share *ShareWithProof
		index int
	}
	resultCh := make(chan *result)
	for index, share := range errByz.Shares {
		if share == nil {
			continue
		}

		index := index
		go func() {
			share, err := getProofsAt(
				ctx, bGetter,
				ipld.MustCidFromNamespacedSha256(roots[index]),
				int(errByz.Index), len(errByz.Shares),
			)
			if err != nil {
				log.Warn("requesting proof failed", "root", roots[index], "err", err)
				return
			}
			resultCh <- &result{share, index}
		}()
	}

	for i := 0; i < len(dah.RowRoots)/2; i++ {
		select {
		case t := <-resultCh:
			sharesWithProof[t.index] = t.share
		case <-ctx.Done():
			return ipld.ErrNodeNotFound
		}
	}

	return &ErrByzantine{
		Index:  uint32(errByz.Index),
		Shares: sharesWithProof,
		Axis:   errByz.Axis,
	}
}
