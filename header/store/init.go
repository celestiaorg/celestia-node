package store

import (
	"context"
	headerpkg "github.com/celestiaorg/celestia-node/pkg/header"

	"github.com/celestiaorg/celestia-node/header"
)

// Init ensures a Store is initialized. If it is not already initialized,
// it initializes the Store by requesting the header with the given hash.
func Init(ctx context.Context, store header.Store, ex header.Exchange, hash headerpkg.Hash) error {
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
