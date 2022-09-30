package share

import (
	"context"
	"fmt"
	"math"

	"github.com/ipfs/go-blockservice"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/tendermint/tendermint/pkg/wrapper"
)

// AddShares erasures and extends shares to blockservice.BlockService using the provided ipld.NodeAdder.
func AddShares(
	ctx context.Context,
	shares []Share,
	adder blockservice.BlockService,
) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("empty data") // empty block is not an empty Data
	}
	squareSize := int(math.Sqrt(float64(len(shares))))
	// create nmt adder wrapping batch adder with calculated size
	bs := BatchSize(squareSize * 2)
	batchAdder := NewNmtNodeAdder(ctx, adder, ipld.MaxSizeBatchOption(bs))
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

// ImportShares imports flattend chunks of data into Extended Data square and saves it in blockservice.BlockService
func ImportShares(
	ctx context.Context,
	shares [][]byte,
	adder blockservice.BlockService) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("ipld: importing empty data")
	}
	squareSize := int(math.Sqrt(float64(len(shares))))
	// create nmt adder wrapping batch adder with calculated size
	bs := BatchSize(squareSize * 2)
	batchAdder := NewNmtNodeAdder(ctx, adder, ipld.MaxSizeBatchOption(bs))
	// create the nmt wrapper to generate row and col commitments
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareSize/2), nmt.NodeVisitor(batchAdder.Visit))
	// recompute the eds
	eds, err := rsmt2d.ImportExtendedDataSquare(shares, DefaultRSMT2DCodec(), tree.Constructor)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	eds.RowRoots()
	// commit the batch to DAG
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
