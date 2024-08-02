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
		params := DefaultParameters()
		params.RecentBlocksCacheSize = 0
		store, err := NewStore(params, t.TempDir())
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		eds, roots := randomEDS(t)
		height := height.Add(1)
		err = store.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		_, err = store.cache.Get(height)
		require.ErrorIs(t, err, cache.ErrCacheMiss)

		withCache, err := store.WithCache("test", 10)
		require.NoError(t, err)
		// load accessor to withCache inner cache by calling GetByHeight
		_, err = withCache.GetByHeight(ctx, height)
		require.NoError(t, err)

		// loaded accessor should be available in both caches
		_, err = withCache.cache.Get(height)
		require.NoError(t, err)
		_, err = store.cache.Get(height)
		require.NoError(t, err)
	})

	t.Run("exists in first cache", func(t *testing.T) {
		store, err := NewStore(DefaultParameters(), t.TempDir())
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		eds, roots := randomEDS(t)
		height := height.Add(1)
		err = store.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		_, err = store.cache.Get(height)
		require.NoError(t, err)

		withCache, err := store.WithCache("test", 10)
		require.NoError(t, err)
		_, err = withCache.GetByHeight(ctx, height)
		require.NoError(t, err)

		_, err = withCache.cache.Second().Get(height)
		require.ErrorIs(t, err, cache.ErrCacheMiss)
	})
}
