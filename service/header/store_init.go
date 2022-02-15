package header

import (
	"context"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// InitStore ensures a Store is initialized. If it is not already initialized,
// it initializes the Store by requesting the header with the given hash.
func InitStore(ctx context.Context, store Store, ex Exchange, hash tmbytes.HexBytes) error {
	_, err := store.Head(ctx)
	switch err {
	default:
		return err
	case ErrNoHead:
		initial, err := ex.RequestByHash(ctx, hash)
		if err != nil {
			return err
		}

		return store.Init(ctx, initial)
	}
}
