package byzantine

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"
	"github.com/ipfs/boxo/blockservice"

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
	sharesAmount := 0

	type tempStruct struct {
		share *ShareWithProof
		index int
		err   error
	}
	tempStructCh := make(chan *tempStruct)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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
			tempStructCh <- &tempStruct{share, index, err}
		}()
	}

	for {
		select {
		case t := <-tempStructCh:
			if t.err != nil {
				log.Errorw("requesting proof failed", "root", string(roots[t.index]), "err", t.err)
				continue
			}
			sharesWithProof[t.index] = t.share
			sharesAmount++
			if sharesAmount == len(dah.RowRoots)/2 {
				return &ErrByzantine{
					Index:  uint32(errByz.Index),
					Shares: sharesWithProof,
					Axis:   errByz.Axis,
				}
			}
		case <-ctx.Done():
			return ipld.ErrNodeNotFound
		}
	}
}
