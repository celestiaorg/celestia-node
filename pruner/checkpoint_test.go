package pruner

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"
)

func TestStoreCheckpoint(t *testing.T) {
	ctx := context.Background()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	c := &checkpoint{
		LastPrunedHeight: 1,
		FailedHeaders:    map[uint64]struct{}{1: {}},
	}

	err := storeCheckpoint(ctx, ds, c)
	require.NoError(t, err)

	c2, err := getCheckpoint(ctx, ds)
	require.NoError(t, err)
	require.Equal(t, c, c2)
}
