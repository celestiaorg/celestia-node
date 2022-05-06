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

// AddShares erasures and extends shares to IPLD DAG using the provided ipld.NodeAdder.
func AddShares(ctx context.Context, shares []Share, adder ipld.NodeAdder) (*rsmt2d.ExtendedDataSquare, error) {
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
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, DefaultRSMT2DCodec(), tree.Constructor)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	eds.RowRoots()
	// commit the batch to ipfs
	return eds, batchAdder.Commit()
}

// ExtractODS returns the original shares of the given ExtendedDataSquare. This
// is a helper function for circumstances where AddShares must be used after the EDS has already
// been generated.
func ExtractODS(eds *rsmt2d.ExtendedDataSquare) []Share {
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

// ExtractEDS takes an EDS and extracts all shares from it in a flattened slice(row by row).
func ExtractEDS(eds *rsmt2d.ExtendedDataSquare) []Share {
	flattenedEDSSize := eds.Width() * eds.Width()
	out := make([][]byte, flattenedEDSSize)
	count := 0
	for i := uint(0); i < eds.Width(); i++ {
		for _, share := range eds.Row(i) {
			out[count] = share
			count++
		}
	}
	return out
}

// batchSize calculates the amount of nodes that are generated from block of 'squareSizes'
// to be batched in one write.
func batchSize(squareSize int) int {
	// (squareSize*2-1) - amount of nodes in a generated binary tree
	// squareSize*2 - the total number of trees, both for rows and cols
	// (squareSize*squareSize) - all the shares
	//
	// Note that while our IPLD tree looks like this:
	// ---X
	// -X---X
	// X-X-X-X
	// X-X-X-X
	// here we count leaves only once: the CIDs are the same for columns and rows
	// and for the last two layers as well:
	return (squareSize*2-1)*squareSize*2 - (squareSize * squareSize)
}
