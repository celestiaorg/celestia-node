package core

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/go-square/square"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

// extendBlock extends the given block data, returning the resulting
// ExtendedDataSquare (EDS). If there are no transactions in the block,
// nil is returned in place of the eds.
func extendBlock(data types.Data, appVersion uint64, options ...nmt.Option) (*rsmt2d.ExtendedDataSquare, error) {
	if app.IsEmptyBlock(data, appVersion) {
		return share.EmptyEDS(), nil
	}

	// Construct the data square from the block's transactions
	dataSquare, err := square.Construct(
		data.Txs.ToSliceOfBytes(),
		appconsts.SquareSizeUpperBound(appVersion),
		appconsts.SubtreeRootThreshold(appVersion),
	)
	if err != nil {
		return nil, err
	}
	return extendShares(shares.ToBytes(dataSquare), options...)
}

func extendShares(s [][]byte, options ...nmt.Option) (*rsmt2d.ExtendedDataSquare, error) {
	// Check that the length of the square is a power of 2.
	if !shares.IsPowerOfTwo(len(s)) {
		return nil, fmt.Errorf("number of shares is not a power of 2: got %d", len(s))
	}
	// here we construct a tree
	// Note: uses the nmt wrapper to construct the tree.
	squareSize := square.Size(len(s))
	return rsmt2d.ComputeExtendedDataSquare(s,
		appconsts.DefaultCodec(),
		wrapper.NewConstructor(uint64(squareSize),
			options...))
}

// storeEDS will only store extended block if it is not empty and doesn't already exist.
func storeEDS(
	ctx context.Context,
	eh *header.ExtendedHeader,
	eds *rsmt2d.ExtendedDataSquare,
	store *store.Store,
	window pruner.AvailabilityWindow,
) error {
	if !pruner.IsWithinAvailabilityWindow(eh.Time(), window) {
		log.Debugw("skipping storage of historic block", "height", eh.Height())
		return nil
	}

	err := store.Put(ctx, eh.DAH, eh.Height(), eds)
	if err == nil {
		log.Debugw("stored EDS for height", "height", eh.Height())
	}
	return err
}
