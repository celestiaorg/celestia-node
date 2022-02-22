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

// PutData posts erasured block data to IPFS using the provided ipld.NodeAdder.
func PutData(ctx context.Context, shares [][]byte, adder ipld.NodeAdder) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("empty data") // empty block is not an empty Data
	}
	squareSize := int(math.Sqrt(float64(len(shares))))
	// create nmt adder wrapping batch adder with calculated size
	bs := batchSize(squareSize * 2)
	batchAdder := NewNmtNodeAdder(ctx, ipld.NewBatch(ctx, adder, ipld.MaxSizeBatchOption(bs)))
	// create the nmt wrapper to generate row and col commitments
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareSize), nmt.NodeVisitor(batchAdder.Visit))
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

// batchSize calculates the amount of nodes that are generated from block of 'squareSizes'
// to be batched in one write.
func batchSize(squareSize int) int {
	// (squareSize*squareSize) - all the shares
	// (squareSize*2-1) - amount of nodes in a generated binary tree
	// *squareSize*2 - multiplier for the amount of trees, both over rows and cols
	//
	// Note that our IPLD tree looks like:
	// ---X
	// -X---X
	// X-X-X-X
	// X-X-X-X
	// So we count leaves two times here for a reason. https://github.com/celestiaorg/celestia-node/issues/183
	return (squareSize*2-1)*squareSize*2 - (squareSize * squareSize)
}
