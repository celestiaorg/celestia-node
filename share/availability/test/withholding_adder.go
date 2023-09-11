package availability_test

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockservice"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

// AddShares erasures and extends shares to blockservice.BlockService using the provided
// ipld.NodeAdder.
func AddAndWithholdShares(
	ctx context.Context,
	shares []share.Share,
	adder blockservice.BlockService,
	recoverable bool,
) (*rsmt2d.ExtendedDataSquare, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("empty data") // empty block is not an empty Data
	}
	squareSize := int(utils.SquareSize(len(shares)))
	withheldSize := squareSize + 1
	// create nmt adder wrapping batch adder with calculated size
	batchAdder := ipld.NewNmtNodeAdder(ctx, adder, ipld.MaxSizeBatchOption(squareSize*2))
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
	err = batchAdder.Commit()
	if err != nil {
		return nil, fmt.Errorf("failure to commit the batch: %w", err)
	}
	dah, err := da.NewDataAvailabilityHeader(eds)
	if err != nil {
		return nil, fmt.Errorf("failure to create data availability header: %w", err)
	}

	// remove blocks from blockservice
	for i := 0; i < withheldSize; i++ {
		for j := 0; j < withheldSize; j++ {
			// leave the last block in the last row and column if data should be left recoverable
			if recoverable && i == withheldSize-1 && j == withheldSize-1 {
				continue
			}
			root, idx := ipld.Translate(&dah, i, j)
			block, err := ipld.GetLeaf(ctx, adder, root, idx, len(dah.RowRoots))
			if err != nil {
				return nil, fmt.Errorf("failure to get leaf: %w", err)
			}
			err = adder.DeleteBlock(ctx, block.Cid())
			if err != nil {
				return nil, fmt.Errorf("failure to delete block: %w", err)
			}
		}
	}
	return eds, nil
}
