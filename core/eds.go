package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/filecoin-project/dagstore"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/pkg/square"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// extendBlock extends the given block data, returning the resulting
// ExtendedDataSquare (EDS). If there are no transactions in the block,
// nil is returned in place of the eds.
func extendBlock(data types.Data, appVersion uint64, options ...nmt.Option) (*rsmt2d.ExtendedDataSquare, error) {
	if app.IsEmptyBlock(data, appVersion) {
		return nil, nil
	}

	// Construct the data square from the block's transactions
	dataSquare, err := square.Construct(data.Txs.ToSliceOfBytes(), appVersion, appconsts.SquareSizeUpperBound(appVersion))
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
func storeEDS(ctx context.Context, datahash share.DataHash, eds *rsmt2d.ExtendedDataSquare, store *eds.Store) error {
	if eds == nil {
		return nil
	}
	err := store.Put(ctx, datahash, eds)
	if errors.Is(err, dagstore.ErrShardExists) {
		// block with given root already exists, return nil
		return nil
	}
	return err
}
