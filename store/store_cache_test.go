package store

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/store/cache"
)

func TestStore_WithCache(t *testing.T) {
	height := atomic.Uint64{}
	height.Store(1)

	t.Run("don't exist in first cache", func(t *testing.T) {
		// create store with no cache
		params := paramsNoCache()
		store, err := NewStore(params, t.TempDir())
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		eds, roots := randomEDS(t)
		height := height.Add(1)
		err = store.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)

		// check that the height is not in the cache (cache was disabled)
		_, err = store.cache.Get(height)
		require.ErrorIs(t, err, cache.ErrCacheMiss)

		cachedStore, err := store.WithCache("test", 10)
		require.NoError(t, err)
		// load accessor to secondary cache by calling GetByHeight on cached store
		acc, err := cachedStore.GetByHeight(ctx, height)
		require.NoError(t, err)
		require.NoError(t, acc.Close())

		// loaded accessor should be available in both original store and wrapped store
		acc, err = store.cache.Get(height)
		require.NoError(t, err)
		require.NoError(t, acc.Close())
		acc, err = cachedStore.combinedCache.Get(height)
		require.NoError(t, err)
		require.NoError(t, acc.Close())
	})

	t.Run("exists in first cache", func(t *testing.T) {
		store, err := NewStore(DefaultParameters(), t.TempDir())
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		eds, roots := randomEDS(t)
		height := height.Add(1)
		err = store.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)

		acc, err := store.cache.Get(height)
		require.NoError(t, err)
		require.NoError(t, acc.Close())

		withCache, err := store.WithCache("test", 10)
		require.NoError(t, err)
		acc, err = withCache.GetByHeight(ctx, height)
		require.NoError(t, err)
		require.NoError(t, acc.Close())

		_, err = withCache.combinedCache.Second().Get(height)
		require.ErrorIs(t, err, cache.ErrCacheMiss)
	})
}

func paramsNoCache() *Parameters {
	params := DefaultParameters()
	params.RecentBlocksCacheSize = 0
	return params
}
