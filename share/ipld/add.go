package ipld

import (
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
)

// AddShares erasures and extends shares to blockservice.BlockService using the provided
// ipld.NodeAdder.
func AddShares(
	ctx context.Context,
	shares []share.Share,
	adder blockservice.BlockService,
) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("empty data") // empty block is not an empty Data
	}
	squareSize := int(utils.SquareSize(len(shares)))
	// create nmt adder wrapping batch adder with calculated size
	batchAdder := NewNmtNodeAdder(ctx, adder, MaxSizeBatchOption(squareSize*2))
	// create the nmt wrapper to generate row and col commitments
	// recompute the eds
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(squareSize),
			nmt.NodeVisitor(batchAdder.Visit)),
	)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	_, err = eds.RowRoots()
	if err != nil {
		return nil, err
	}
	// commit the batch to ipfs
	return eds, batchAdder.Commit()
}

// ImportShares imports flattened chunks of data into Extended Data square and saves it in
// blockservice.BlockService
func ImportShares(
	ctx context.Context,
	shares [][]byte,
	adder blockservice.BlockService) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("ipld: importing empty data")
	}
	squareSize := int(utils.SquareSize(len(shares)))
	// create nmt adder wrapping batch adder with calculated size
	batchAdder := NewNmtNodeAdder(ctx, adder, MaxSizeBatchOption(squareSize*2))
	// recompute the eds
	eds, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(squareSize/2),
			nmt.NodeVisitor(batchAdder.Visit)),
	)
	if err != nil {
		return nil, fmt.Errorf("failure to recompute the extended data square: %w", err)
	}
	// compute roots
	_, err = eds.RowRoots()
	if err != nil {
		return nil, err
	}
	// commit the batch to DAG
	return eds, batchAdder.Commit()
}

func ImportEDS(ctx context.Context, square *rsmt2d.ExtendedDataSquare, adder blockservice.BlockService) error {
	shares := share.ExtractEDS(square)
	_, err := ImportShares(ctx, shares, adder)
	return err
}
