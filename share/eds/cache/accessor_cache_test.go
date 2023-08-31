package cache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestAccessorCache(t *testing.T) {
	t.Run("add / get item from cache", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		key := shard.KeyFromString("key")
		mock := &mockAccessorProvider{}
		loaded, err := cache.GetOrLoad(ctx, key, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// check if item exists
		got, err := cache.Get(key)
		require.NoError(t, err)
		require.True(t, loaded == got)
	})

	t.Run("get blockstore from accessor", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		key := shard.KeyFromString("key")
		mock := &mockAccessorProvider{}
		accessor, err := cache.GetOrLoad(ctx, key, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// check if item exists
		_, err = cache.Get(key)
		require.NoError(t, err)

		// blockstore should be created only after first request
		require.Equal(t, 0, mock.returnedBs)

		// try to get blockstore
		_, err = accessor.Blockstore()
		require.NoError(t, err)

		// second call to blockstore should return same blockstore
		bs, err := accessor.Blockstore()
		require.Equal(t, 1, mock.returnedBs)
		require.NoError(t, err)

		// check that closed blockstore don't close accessor
		err = bs.Close()
		require.NoError(t, err)
		require.False(t, mock.isClosed)
	})

	t.Run("remove an item", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		key := shard.KeyFromString("key")
		mock := &mockAccessorProvider{}
		_, err = cache.GetOrLoad(ctx, key, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		err = cache.Remove(key)
		require.NoError(t, err)

		// accessor should be closed on removal
		require.True(t, mock.isClosed)

		// check if item exists
		_, err = cache.Get(key)
		require.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("successive reads should read the same data", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		key := shard.KeyFromString("key")
		mock := &mockAccessorProvider{data: []byte("test")}
		accessor, err := cache.GetOrLoad(ctx, key, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		loaded, err := io.ReadAll(accessor.ReadCloser())
		require.NoError(t, err)
		require.Equal(t, mock.data, loaded)

		for i := 0; i < 2; i++ {
			accessor, err = cache.Get(key)
			require.NoError(t, err)
			got, err := io.ReadAll(accessor.ReadCloser())
			require.NoError(t, err)
			require.Equal(t, mock.data, got)
		}
	})

	t.Run("removed by eviction", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		key := shard.KeyFromString("key")
		mock := &mockAccessorProvider{}
		_, err = cache.GetOrLoad(ctx, key, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// add second item
		key2 := shard.KeyFromString("key2")
		_, err = cache.GetOrLoad(ctx, key2, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// give cache time to evict an item
		time.Sleep(time.Millisecond * 100)
		// accessor should be closed on removal by eviction
		require.True(t, mock.isClosed)

		// check if item evicted
		_, err = cache.Get(key)
		require.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("close on accessor is noop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		key := shard.KeyFromString("key")
		mock := &mockAccessorProvider{}
		_, err = cache.GetOrLoad(ctx, key, func(ctx context.Context, key shard.Key) (AccessorProvider, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// check if item exists
		accessor, err := cache.Get(key)
		require.NoError(t, err)
		require.NotNil(t, accessor)

		// close on reader should be noop
		err = accessor.ReadCloser().Close()
		require.NoError(t, err)

		// close on blockstore should be noop
		bs, err := accessor.Blockstore()
		require.NoError(t, err)
		err = bs.Close()
		require.NoError(t, err)

		// check that close was not performed on accessor
		require.False(t, mock.isClosed)
	})
}

type mockAccessorProvider struct {
	data       []byte
	isClosed   bool
	returnedBs int
}

func (m *mockAccessorProvider) Reader() io.Reader {
	return bytes.NewBuffer(m.data)
}

func (m *mockAccessorProvider) Blockstore() (dagstore.ReadBlockstore, error) {
	if m.returnedBs > 0 {
		return nil, errors.New("blockstore already returned")
	}
	m.returnedBs++
	return rbsMock{}, nil
}

func (m *mockAccessorProvider) Close() error {
	if m.isClosed {
		return errors.New("already closed")
	}
	m.isClosed = true
	return nil
}

// rbsMock is a dagstore.ReadBlockstore mock
type rbsMock struct{}

func (r rbsMock) Has(context.Context, cid.Cid) (bool, error) {
	panic("implement me")
}

func (r rbsMock) Get(_ context.Context, _ cid.Cid) (blocks.Block, error) {
	panic("implement me")
}

func (r rbsMock) GetSize(context.Context, cid.Cid) (int, error) {
	panic("implement me")
}

func (r rbsMock) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	panic("implement me")
}

func (r rbsMock) HashOnRead(bool) {
	panic("implement me")
}
