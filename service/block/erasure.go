package block

import (
	"math"

	"github.com/celestiaorg/celestia-core/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"
)

// extendBlockData erasure codes the given raw block's data and returns the
// erasure coded block data upon success.
func extendBlockData(raw *Raw) (*ExtendedBlockData, error) {
	namespacedShares, _ := raw.Data.ComputeShares()
	shares := namespacedShares.RawShares()

	// create the nmt wrapper to generate row and col commitments
	squareSize := squareSize64(len(namespacedShares))
	tree := wrapper.NewErasuredNamespacedMerkleTree(squareSize)

	// compute extended square
	extendedDataSquare, err := rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	if err != nil {
		return nil, err
	}

	return extendedDataSquare, nil
}

// squareSize64 computes the square size as a uint64 from
// the given length of shares.
func squareSize64(length int) uint64 {
	return uint64(math.Sqrt(float64(length)))
}
