package store

import (
	"context"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/header"
)

// Init ensures a Store is initialized. If it is not already initialized,
// it initializes the Store by requesting the header with the given hash.
func Init(ctx context.Context, store header.Store, ex header.Exchange, hash tmbytes.HexBytes) error {
	_, err := store.Head(ctx)
	switch err {
	default:
		return err
	case header.ErrNoHead:
		initial, err := ex.Get(ctx, hash)
		if err != nil {
			return err
		}

		return store.Init(ctx, initial)
	}
}
