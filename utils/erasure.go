package utils

import (
	"context"
	"math"

	format "github.com/ipfs/go-ipld-format"

	"github.com/tendermint/tendermint/pkg/wrapper"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

// ExtendBlock erasure codes the given raw block's data and returns the
// erasure coded block data upon success.
func ExtendBlock(block *core.Block, dag format.DAGService) (*rsmt2d.ExtendedDataSquare, error) {
	namespacedShares, _ := block.Data.ComputeShares()
	shares := namespacedShares.RawShares()

	// create nmt adder wrapping batch adder
	batchAdder := ipld.NewNmtNodeAdder(context.Background(), format.NewBatch(context.Background(), dag))

	// create the nmt wrapper to generate row and col commitments
	squareSize := squareSize64(len(namespacedShares))
	tree := wrapper.NewErasuredNamespacedMerkleTree(squareSize, nmt.NodeVisitor(batchAdder.Visit))

	// compute extended square
	return rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
}

// squareSize64 computes the square size as uint64 from
// the given length of shares.
func squareSize64(length int) uint64 {
	return uint64(math.Sqrt(float64(length)))
}
