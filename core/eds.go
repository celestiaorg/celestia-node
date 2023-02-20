package core

import (
	"context"

	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	appshares "github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// extendBlock extends the given block data, returning the resulting
// ExtendedDataSquare (EDS). If there are no transactions in the block,
// nil is returned in place of the eds.
func extendBlock(data types.Data) (*rsmt2d.ExtendedDataSquare, error) {
	if len(data.Txs) == 0 {
		return nil, nil
	}
	shares, err := appshares.Split(data, true)
	if err != nil {
		return nil, err
	}
	size := utils.SquareSize(len(shares))
	return da.ExtendShares(size, appshares.ToBytes(shares))
}

// storeEDS will only store extended block if it is not empty and doesn't already exist.
func storeEDS(ctx context.Context, hash share.DataHash, eds *rsmt2d.ExtendedDataSquare, store *eds.Store) error {
	if eds == nil {
		return nil
	}
	has, err := store.Has(ctx, hash)
	if err != nil {
		return err
	}
	if has {
		log.Debugw("hash already exists in eds.Store", "hash", hash.String())
		return nil
	}
	return store.Put(ctx, hash, eds)
}
