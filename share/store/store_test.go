package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"
	"go.uber.org/atomic"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/store/cache"
)

//TODO: add benchmarks for store

func TestEDSStore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	edsStore, err := NewStore(DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	// disable cache
	edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})
	height := atomic.NewUint64(100)

	// PutRegistersShard tests if Put registers the shard on the underlying DAGStore
	t.Run("Put", func(t *testing.T) {
		eds, dah := randomEDS(t)
		height := height.Add(1)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		// file should become available by hash
		has, err := edsStore.HasByHash(ctx, dah.Hash())
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

		eds, dah := randomEDS(t)
		height := height.Add(1)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		// file should be cached after put
		f, err = edsStore.cache.Get(height)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		// check that cached file is the same eds
		fromFile, err := f.EDS(ctx)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		require.True(t, eds.Equals(fromFile))
	})

	t.Run("Second Put should be noop", func(t *testing.T) {
		eds, dah := randomEDS(t)
		height := height.Add(1)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	})

	t.Run("GetByHeight", func(t *testing.T) {
		eds, dah := randomEDS(t)
		height := height.Add(1)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = edsStore.GetByHeight(ctx, height)
		require.NoError(t, err)

		fromFile, err := f.EDS(ctx)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		require.True(t, eds.Equals(fromFile))
	})

	t.Run("GetByDataHash", func(t *testing.T) {
		eds, dah := randomEDS(t)
		height := height.Add(1)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		f, err = edsStore.GetByHash(ctx, dah.Hash())
		require.NoError(t, err)

		fromFile, err := f.EDS(ctx)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		require.True(t, eds.Equals(fromFile))
	})

	t.Run("Does not exist", func(t *testing.T) {
		_, dah := randomEDS(t)
		height := height.Add(1)

		has, err := edsStore.HasByHash(ctx, dah.Hash())
		require.NoError(t, err)
		require.False(t, has)

		has, err = edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, has)

		_, err = edsStore.GetByHeight(ctx, height)
		require.ErrorIs(t, err, ErrNotFound)

		_, err = edsStore.GetByHash(ctx, dah.Hash())
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("Remove", func(t *testing.T) {
		// removing file that not exists should be noop
		missingHeight := height.Add(1)
		err := edsStore.Remove(ctx, missingHeight)
		require.NoError(t, err)

		eds, dah := randomEDS(t)
		height := height.Add(1)
		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		err = edsStore.Remove(ctx, height)
		require.NoError(t, err)

		// file should be removed from cache
		_, err = edsStore.cache.Get(height)
		require.ErrorIs(t, err, cache.ErrCacheMiss)

		// file should not be accessible by hash
		has, err := edsStore.HasByHash(ctx, dah.Hash())
		require.NoError(t, err)
		require.False(t, has)

		// file should not be accessible by height
		has, err = edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, has)
	})

	t.Run("empty EDS returned by hash", func(t *testing.T) {
		eds := share.EmptyExtendedDataSquare()
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)

		// assert that the empty file exists
		has, err := edsStore.HasByHash(ctx, dah.Hash())
		require.NoError(t, err)
		require.True(t, has)

		// assert that the empty file is, in fact, empty
		f, err := edsStore.GetByHash(ctx, dah.Hash())
		require.NoError(t, err)
		require.True(t, f.DataHash().IsEmptyRoot())
	})

	t.Run("empty EDS returned by height", func(t *testing.T) {
		eds := share.EmptyExtendedDataSquare()
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)
		height := height.Add(1)

		// assert that the empty file exists
		has, err := edsStore.HasByHeight(ctx, height)
		require.NoError(t, err)
		require.False(t, has)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.True(t, f.DataHash().IsEmptyRoot())
		require.NoError(t, f.Close())

		// assert that the empty file can be accessed by height
		f, err = edsStore.GetByHeight(ctx, height)
		require.NoError(t, err)
		require.True(t, f.DataHash().IsEmptyRoot())
	})

	t.Run("empty EDS are persisted", func(t *testing.T) {
		dir := t.TempDir()
		edsStore, err := NewStore(DefaultParameters(), dir)
		require.NoError(t, err)

		eds := share.EmptyExtendedDataSquare()
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)
		from, to := 10, 20

		// store empty EDSs
		for i := from; i <= to; i++ {
			f, err := edsStore.Put(ctx, dah.Hash(), uint64(i), eds)
			require.NoError(t, err)
			require.NoError(t, f.Close())
		}

		// close and reopen the store to ensure that the empty files are persisted
		require.NoError(t, edsStore.Close())
		edsStore, err = NewStore(DefaultParameters(), dir)
		require.NoError(t, err)

		// assert that the empty files restored from disk
		for i := from; i <= to; i++ {
			f, err := edsStore.GetByHeight(ctx, uint64(i))
			require.NoError(t, err)
			require.True(t, f.DataHash().IsEmptyRoot())
			require.NoError(t, f.Close())
		}
	})
}

func BenchmarkStore(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	edsStore, err := NewStore(DefaultParameters(), b.TempDir())
	require.NoError(b, err)

	eds := edstest.RandEDS(b, 128)
	require.NoError(b, err)

	// BenchmarkStore/bench_put_128-10         	      27	  43968818 ns/op (~43ms)
	b.Run("put 128", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			h := share.DataHash(rand.Bytes(5))
			f, _ := edsStore.Put(ctx, h, uint64(i), eds)
			_ = f.Close()
		}
	})

	// read 128 EDSs does not read full EDS, but only the header
	// BenchmarkStore/bench_read_128-10         	   82766	     14678 ns/op (~14ms)
	b.Run("open by height, 128", func(b *testing.B) {
		edsStore, err := NewStore(DefaultParameters(), b.TempDir())
		require.NoError(b, err)

		// disable cache
		edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})

		dah, err := share.NewRoot(eds)
		require.NoError(b, err)

		height := uint64(1984)
		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(b, err)
		require.NoError(b, f.Close())

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f, err := edsStore.GetByHeight(ctx, height)
			require.NoError(b, err)
			_ = f.Close()
		}
	})

	// BenchmarkStore/open_by_hash,_128-10         	   72921	     16799 ns/op (~16ms)
	b.Run("open by hash, 128", func(b *testing.B) {
		edsStore, err := NewStore(DefaultParameters(), b.TempDir())
		require.NoError(b, err)

		// disable cache
		edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})

		dah, err := share.NewRoot(eds)
		require.NoError(b, err)

		height := uint64(1984)
		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(b, err)
		require.NoError(b, f.Close())

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			f, err := edsStore.GetByHash(ctx, dah.Hash())
			require.NoError(b, err)
			_ = f.Close()
		}
	})
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.Root) {
	eds := edstest.RandEDS(t, 4)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)

	return eds, dah
}
