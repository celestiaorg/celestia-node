package core

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/square"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// extendBlock extends the given block data, returning the resulting
// ExtendedDataSquare (EDS). If there are no transactions in the block,
// nil is returned in place of the eds.
func extendBlock(data types.Data) (*rsmt2d.ExtendedDataSquare, error) {
	if len(data.Txs) == 0 && data.SquareSize == uint64(1) {
		return nil, nil
	}

	sqr, err := square.Construct(data.Txs.ToSliceOfBytes(), appconsts.LatestVersion, share.MaxSquareSize)
	if err != nil {
		return nil, err
	}

	shares := make([][]byte, len(sqr))
	for i, s := range sqr {
		shares[i] = s.ToBytes()
	}

	return da.ExtendShares(shares)
}

// storeEDS will only store extended block if it is not empty and doesn't already exist.
func storeEDS(ctx context.Context, hash share.DataHash, eds *rsmt2d.ExtendedDataSquare, store *eds.Store) error {
	if eds == nil {
		return nil
	}
	err := store.Put(ctx, hash, eds)
	if errors.Is(err, dagstore.ErrShardExists) {
		// block with given root already exists, return nil
		return nil
	}
	return err
}
