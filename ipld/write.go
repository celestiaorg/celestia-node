package ipld

import (
	"context"
	"fmt"
	"math"

	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/tendermint/tendermint/pkg/wrapper"
)

// BatchSize defines an amount of IPLD Nodes to be buffered and written at once.
// TODO(@Wondertan): Should rely on biggest extended block size param instead
const BatchSize = 16384 // (128*128 so we flush big blocks in one IO write) //

// PutData posts erasured block data to IPFS using the provided ipld.NodeAdder.
func PutData(ctx context.Context, shares [][]byte, adder ipld.NodeAdder) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("empty data") // empty block is not an empty Data
	}
	// create nmt adder wrapping batch adder
	batchAdder := NewNmtNodeAdder(ctx, ipld.NewBatch(ctx, adder, ipld.MaxSizeBatchOption(BatchSize)))
	// create the nmt wrapper to generate row and col commitments
	squareSize := uint64(math.Sqrt(float64(len(shares))))
	tree := wrapper.NewErasuredNamespacedMerkleTree(squareSize, nmt.NodeVisitor(batchAdder.Visit))
	// recompute the eds
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	eds.RowRoots()
	// commit the batch to ipfs
	return eds, batchAdder.Commit()
}

// ExtractODSShares returns the original shares of the given ExtendedDataSquare. This
// is a helper function for circumstances where PutData must be used after the EDS has already
// been generated.
func ExtractODSShares(eds *rsmt2d.ExtendedDataSquare) [][]byte {
	origWidth := eds.Width() / 2
	origShares := make([][]byte, origWidth*origWidth)
	for i := uint(0); i < origWidth; i++ {
		row := eds.Row(i)
		for j := uint(0); j < origWidth; j++ {
			origShares[(i*origWidth)+j] = row[j]
		}
	}
	return origShares
}
