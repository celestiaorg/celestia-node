package store

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store/cache"
)

func TestEDSStore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	dir := t.TempDir()
	edsStore, err := NewStore(paramsNoCache(), dir)
	require.NoError(t, err)

	// disable cache
	edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})
	height := atomic.Uint64{}
	height.Store(100)

	t.Run("Put", func(t *testing.T) {
		eds, roots := randomEDS(t)
		height := height.Add(1)

		err := edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		// file should become available by hash
		has, err := edsStore.HasByHash(ctx, roots.Hash())
		require.NoError(t, err)
		require.True(t, has)

		// file should become available by height
		has, err = edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("Cached after Put", func(t *testing.T) {
		edsStore, err := NewStore(DefaultParameters(), t.TempDir())
		require.NoError(t, err)

		eds, roots := randomEDS(t)
		height := height.Add(1)

		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		// file should be cached after put
		f, err := edsStore.cache.Get(height)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		// check that cached file is the same eds
		fromFile, err := f.Shares(ctx)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		expected := eds.FlattenedODS()
		require.Equal(t, expected, fromFile)
	})

	t.Run("Second Put should be noop", func(t *testing.T) {
		eds, roots := randomEDS(t)
		height := height.Add(1)

		err := edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)
		// TODO: check amount of files in the store after the second Put
		// after store supports listing
	})

	t.Run("GetByHeight", func(t *testing.T) {
		eds, roots := randomEDS(t)
		height := height.Add(1)

		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		f, err := edsStore.GetByHeight(ctx, height)
		require.NoError(t, err)

		// check that cached file is the same eds
		fromFile, err := f.Shares(ctx)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		expected := eds.FlattenedODS()
		require.Equal(t, expected, fromFile)
	})

	t.Run("GetByHash", func(t *testing.T) {
		eds, roots := randomEDS(t)
		height := height.Add(1)

		err := edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		f, err := edsStore.GetByHash(ctx, roots.Hash())
		require.NoError(t, err)

		// check that cached file is the same eds
		fromFile, err := f.Shares(ctx)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		expected := eds.FlattenedODS()
		require.Equal(t, expected, fromFile)
	})

	t.Run("Does not exist", func(t *testing.T) {
		_, roots := randomEDS(t)
		height := height.Add(1)

		has, err := edsStore.HasByHash(ctx, roots.Hash())
		require.NoError(t, err)
		require.False(t, has)

		has, err = edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, has)

		_, err = edsStore.GetByHeight(ctx, height)
		require.ErrorIs(t, err, ErrNotFound)

		_, err = edsStore.GetByHash(ctx, roots.Hash())
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("Remove", func(t *testing.T) {
		edsStore, err := NewStore(DefaultParameters(), t.TempDir())
		require.NoError(t, err)

		// removing file that does not exist should be noop
		missingHeight := height.Add(1)
		err = edsStore.Remove(ctx, missingHeight, share.DataHash{0x01, 0x02})
		require.NoError(t, err)

		eds, roots := randomEDS(t)
		height := height.Add(1)
		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		err = edsStore.Remove(ctx, height, roots.Hash())
		require.NoError(t, err)

		// file should be removed from cache
		_, err = edsStore.cache.Get(height)
		require.ErrorIs(t, err, cache.ErrCacheMiss)

		// file should not be accessible by hash
		has, err := edsStore.HasByHash(ctx, roots.Hash())
		require.NoError(t, err)
		require.False(t, has)

		// subsequent remove should be noop
		err = edsStore.Remove(ctx, height, roots.Hash())
		require.NoError(t, err)

		// file should not be accessible by height
		has, err = edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, has)
	})

	t.Run("empty EDS returned by hash", func(t *testing.T) {
		eds := share.EmptyEDS()
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)

		// assert that the empty file exists
		has, err := edsStore.HasByHash(ctx, roots.Hash())
		require.NoError(t, err)
		require.True(t, has)

		// assert that the empty file is, in fact, empty
		f, err := edsStore.GetByHash(ctx, roots.Hash())
		require.NoError(t, err)
		hash, err := f.DataHash(ctx)
		require.NoError(t, err)
		require.True(t, hash.IsEmptyEDS())
	})

	t.Run("empty EDS returned by height", func(t *testing.T) {
		eds := share.EmptyEDS()
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		height := height.Add(1)

		// assert that the empty file exists
		has, err := edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, has)

		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		// assert that the empty file can be accessed by height
		f, err := edsStore.GetByHeight(ctx, height)
		require.NoError(t, err)
		hash, err := f.DataHash(ctx)
		require.NoError(t, err)
		require.True(t, hash.IsEmptyEDS())
		require.NoError(t, f.Close())
	})

	t.Run("empty EDS are persisted", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(DefaultParameters(), dir)
		require.NoError(t, err)

		eds := share.EmptyEDS()
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		from, to := 10, 20

		// store empty EDSs
		for i := from; i <= to; i++ {
			err := edsStore.Put(ctx, roots, uint64(i), eds)
			require.NoError(t, err)
		}

		// close and reopen the store to ensure that the empty files are persisted
		require.NoError(t, edsStore.Stop(ctx))
		edsStore, err = NewStore(DefaultParameters(), dir)
		require.NoError(t, err)

		// assert that the empty files restored from disk
		for i := from; i <= to; i++ {
			f, err := edsStore.GetByHeight(ctx, uint64(i))
			require.NoError(t, err)
			hash, err := f.DataHash(ctx)
			require.NoError(t, err)
			require.True(t, hash.IsEmptyEDS())
			require.NoError(t, f.Close())
		}
	})

	t.Run("reopen", func(t *testing.T) {
		// tests that store can be reopened
		err = edsStore.Stop(ctx)
		require.NoError(t, err)

		edsStore, err = NewStore(paramsNoCache(), dir)
		require.NoError(t, err)
	})
}

func BenchmarkStore(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	eds := edstest.RandEDS(b, 128)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(b, err)

	// BenchmarkStore/put_128-16         	     186	   6623266 ns/op
	b.Run("put 128", func(b *testing.B) {
		edsStore, err := NewStore(paramsNoCache(), b.TempDir())
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			roots := edstest.RandomAxisRoots(b, 1)
			_ = edsStore.Put(ctx, roots, uint64(i), eds)
		}
	})

	// read 128 EDSs does not read full EDS, but only the header
	// BenchmarkStore/open_by_height,_128-16         	 1585693	       747.6 ns/op (~7mcs)
	b.Run("open by height, 128", func(b *testing.B) {
		edsStore, err := NewStore(paramsNoCache(), b.TempDir())
		require.NoError(b, err)

		height := uint64(1984)
		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f, err := edsStore.GetByHeight(ctx, height)
			require.NoError(b, err)
			require.NoError(b, f.Close())
		}
	})

	// BenchmarkStore/open_by_hash,_128-16           	 1240942	       945.9 ns/op (~9mcs)
	b.Run("open by hash, 128", func(b *testing.B) {
		edsStore, err := NewStore(DefaultParameters(), b.TempDir())
		require.NoError(b, err)

		height := uint64(1984)
		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(b, err)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f, err := edsStore.GetByHash(ctx, roots.Hash())
			require.NoError(b, err)
			require.NoError(b, f.Close())
		}
	})
}

func randomEDS(t testing.TB) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)

	return eds, roots
}
