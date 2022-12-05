package store

import (
	"context"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// Init ensures a Store is initialized. If it is not already initialized,
// it initializes the Store by requesting the header with the given hash.
func Init[H header.Header](ctx context.Context, store header.Store[H], ex header.Exchange[H], hash header.Hash) error {
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
