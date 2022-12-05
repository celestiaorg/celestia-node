package store

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// NewTestStore creates initialized and started in memory header Store which is useful for testing.
func NewTestStore(ctx context.Context, t *testing.T, head *header.DummyHeader) header.Store[*header.DummyHeader] {
	store, err := NewStoreWithHead(ctx, sync.MutexWrap(datastore.NewMapDatastore()), head)
	require.NoError(t, err)

	err = store.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := store.Stop(ctx)
		require.NoError(t, err)
	})
	return store
}
