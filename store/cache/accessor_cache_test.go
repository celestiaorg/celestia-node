package cache

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestAccessorCache(t *testing.T) {
	t.Run("add / has / get item from cache", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{
			data: []byte("test_data"),
		}
		loaded, err := cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)
		reader, err := loaded.Reader()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, mock.data, data)
		err = loaded.Close()
		require.NoError(t, err)

		// check if item exists
		has := cache.Has(height)
		require.True(t, has)
		got, err := cache.Get(height)
		require.NoError(t, err)
		reader, err = got.Reader()
		require.NoError(t, err)
		data, err = io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, mock.data, data)
		err = got.Close()
		require.NoError(t, err)
	})

	t.Run("get reader from accessor", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{}
		accessor, err := cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// check if item exists
		_, err = cache.Get(height)
		require.NoError(t, err)

		// try to get reader
		_, err = accessor.Reader()
		require.NoError(t, err)
	})

	t.Run("remove an item", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{}
		ac, err := cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)
		err = ac.Close()
		require.NoError(t, err)

		err = cache.Remove(height)
		require.NoError(t, err)

		// accessor should be closed on removal
		mock.checkClosed(t, true)

		// check if item exists
		has := cache.Has(height)
		require.False(t, has)
		_, err = cache.Get(height)
		require.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("successive reads should read the same data", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{data: []byte("test")}
		accessor, err := cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)

		reader, err := accessor.Reader()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, mock.data, data)

		for range 2 {
			accessor, err = cache.Get(height)
			require.NoError(t, err)
			reader, err := accessor.Reader()
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, mock.data, data)
		}
	})

	t.Run("removed by eviction", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{}
		ac1, err := cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)
		err = ac1.Close()
		require.NoError(t, err)

		// add second item
		height2 := uint64(2)
		ac2, err := cache.GetOrLoad(ctx, height2, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)
		err = ac2.Close()
		require.NoError(t, err)

		// accessor should be closed on removal by eviction
		mock.checkClosed(t, true)

		// first item should be evicted from cache
		has := cache.Has(height)
		require.False(t, has)
		_, err = cache.Get(height)
		require.ErrorIs(t, err, ErrCacheMiss)
	})

	t.Run("close on accessor is not closing underlying accessor", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{}
		_, err = cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// check if item exists
		accessor, err := cache.Get(height)
		require.NoError(t, err)
		require.NotNil(t, accessor)

		// close on returned accessor should not close inner accessor
		err = accessor.Close()
		require.NoError(t, err)

		// check that close was not performed on inner accessor
		mock.checkClosed(t, false)
	})

	t.Run("close on accessor should wait all readers to finish", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height := uint64(1)
		mock := &mockAccessor{}
		accessor1, err := cache.GetOrLoad(ctx, height, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock, nil
		})
		require.NoError(t, err)

		// create second readers
		accessor2, err := cache.Get(height)
		require.NoError(t, err)

		// initialize close
		done := make(chan struct{})
		go func() {
			err := cache.Remove(height)
			require.NoError(t, err)
			close(done)
		}()

		// close on first reader and check that it is not enough to release the inner accessor
		err = accessor1.Close()
		require.NoError(t, err)
		mock.checkClosed(t, false)

		// second close from same reader should not release accessor either
		err = accessor1.Close()
		require.NoError(t, err)
		mock.checkClosed(t, false)

		// reads for item that is being evicted should result in ErrCacheMiss
		_, err = cache.Get(height)
		require.ErrorIs(t, err, ErrCacheMiss)

		// close second reader and wait for accessor to be closed
		err = accessor2.Close()
		require.NoError(t, err)
		// wait until close is performed on accessor
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("timeout reached")
		}

		// item will be removed
		mock.checkClosed(t, true)
	})

	t.Run("slow reader should not block eviction", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cache, err := NewAccessorCache("test", 1)
		require.NoError(t, err)

		// add accessor to the cache
		height1 := uint64(1)
		mock1 := &mockAccessor{}
		accessor1, err := cache.GetOrLoad(ctx, height1, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock1, nil
		})
		require.NoError(t, err)

		// add second accessor, to trigger eviction of the first one
		height2 := uint64(2)
		mock2 := &mockAccessor{}
		accessor2, err := cache.GetOrLoad(ctx, height2, func(ctx context.Context) (eds.AccessorStreamer, error) {
			return mock2, nil
		})
		require.NoError(t, err)

		// first accessor should be evicted from cache
		_, err = cache.Get(height1)
		require.ErrorIs(t, err, ErrCacheMiss)

		// first accessor should not be closed before all refs are released by Close() is calls.
		mock1.checkClosed(t, false)

		// after Close() is called on first accessor, it is free to get closed
		err = accessor1.Close()
		require.NoError(t, err)
		mock1.checkClosed(t, true)

		// after Close called on second accessor, it should stay in cache (not closed)
		err = accessor2.Close()
		require.NoError(t, err)
		mock2.checkClosed(t, false)
	})
}

type mockAccessor struct {
	m        sync.Mutex
	data     []byte
	isClosed bool
}

func (m *mockAccessor) Size(context.Context) (int, error) {
	panic("implement me")
}

func (m *mockAccessor) DataHash(context.Context) (share.DataHash, error) {
	panic("implement me")
}

func (m *mockAccessor) AxisRoots(context.Context) (*share.AxisRoots, error) {
	panic("implement me")
}

func (m *mockAccessor) Sample(context.Context, shwap.SampleCoords) (shwap.Sample, error) {
	panic("implement me")
}

func (m *mockAccessor) AxisHalf(context.Context, rsmt2d.Axis, int) (shwap.AxisHalf, error) {
	panic("implement me")
}

func (m *mockAccessor) RowNamespaceData(context.Context, libshare.Namespace, int) (shwap.RowNamespaceData, error) {
	panic("implement me")
}

func (m *mockAccessor) RangeNamespaceData(
	_ context.Context,
	_, _ int,
) (shwap.RangeNamespaceData, error) {
	panic("implement me")
}

func (m *mockAccessor) Shares(context.Context) ([]libshare.Share, error) {
	panic("implement me")
}

func (m *mockAccessor) Reader() (io.Reader, error) {
	m.m.Lock()
	defer m.m.Unlock()
	return bytes.NewBuffer(m.data), nil
}

func (m *mockAccessor) Close() error {
	m.m.Lock()
	defer m.m.Unlock()
	if m.isClosed {
		return errors.New("already closed")
	}
	m.isClosed = true
	return nil
}

func (m *mockAccessor) checkClosed(t *testing.T, expected bool) {
	// item will be removed async in background, give it some time to settle
	time.Sleep(time.Millisecond * 100)
	m.m.Lock()
	defer m.m.Unlock()
	require.Equal(t, expected, m.isClosed)
}
