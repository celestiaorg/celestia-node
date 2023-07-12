package eds

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestEDSStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	// PutRegistersShard tests if Put registers the shard on the underlying DAGStore
	t.Run("PutRegistersShard", func(t *testing.T) {
		eds, dah := randomEDS(t)

		// shard hasn't been registered yet
		has, err := edsStore.Has(ctx, dah.Hash())
		assert.False(t, has)
		assert.NoError(t, err)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		assert.NoError(t, err)

		_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
		assert.NoError(t, err)
	})

	// PutIndexesEDS ensures that Putting an EDS indexes it into the car index
	t.Run("PutIndexesEDS", func(t *testing.T) {
		eds, dah := randomEDS(t)

		stat, _ := edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
		assert.False(t, stat.Exists)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		assert.NoError(t, err)

		stat, err = edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
		assert.True(t, stat.Exists)
		assert.NoError(t, err)
	})

	// GetCAR ensures that the reader returned from GetCAR is capable of reading the CAR header and
	// ODS.
	t.Run("GetCAR", func(t *testing.T) {
		eds, dah := randomEDS(t)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		r, err := edsStore.GetCAR(ctx, dah.Hash())
		assert.NoError(t, err)
		carReader, err := car.NewCarReader(r)
		assert.NoError(t, err)

		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				original := eds.GetCell(uint(i), uint(j))
				block, err := carReader.Next()
				assert.NoError(t, err)
				assert.Equal(t, original, share.GetData(block.RawData()))
			}
		}
	})

	t.Run("item not exist", func(t *testing.T) {
		root := share.DataHash{1}
		_, err := edsStore.GetCAR(ctx, root)
		assert.ErrorIs(t, err, ErrNotFound)

		_, err = edsStore.GetDAH(ctx, root)
		assert.ErrorIs(t, err, ErrNotFound)

		_, err = edsStore.CARBlockstore(ctx, root)
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("Remove", func(t *testing.T) {
		eds, dah := randomEDS(t)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		// assert that file now exists
		_, err = os.Stat(edsStore.basepath + blocksPath + dah.String())
		assert.NoError(t, err)

		err = edsStore.Remove(ctx, dah.Hash())
		assert.NoError(t, err)

		// shard should no longer be registered on the dagstore
		_, err = edsStore.dgstr.GetShardInfo(shard.KeyFromString(dah.String()))
		assert.Error(t, err, "shard not found")

		// shard should have been dropped from the index, which also removes the file under /index/
		indexStat, err := edsStore.carIdx.StatFullIndex(shard.KeyFromString(dah.String()))
		assert.NoError(t, err)
		assert.False(t, indexStat.Exists)

		// file no longer exists
		_, err = os.Stat(edsStore.basepath + blocksPath + dah.String())
		assert.ErrorContains(t, err, "no such file or directory")
	})

	t.Run("Has", func(t *testing.T) {
		eds, dah := randomEDS(t)

		ok, err := edsStore.Has(ctx, dah.Hash())
		assert.NoError(t, err)
		assert.False(t, ok)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		assert.NoError(t, err)

		ok, err = edsStore.Has(ctx, dah.Hash())
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("BlockstoreCache", func(t *testing.T) {
		eds, dah := randomEDS(t)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		// key isnt in cache yet, so get returns errCacheMiss
		shardKey := shard.KeyFromString(dah.String())
		_, err = edsStore.cache.Get(shardKey)
		assert.ErrorIs(t, err, errCacheMiss)

		// now get it, so that the key is in the cache
		_, err = edsStore.CARBlockstore(ctx, dah.Hash())
		assert.NoError(t, err)
		_, err = edsStore.cache.Get(shardKey)
		assert.NoError(t, err, errCacheMiss)
	})
}

// TestEDSStore_GC verifies that unused transient shards are collected by the GC periodically.
func TestEDSStore_GC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	edsStore.gcInterval = time.Second
	require.NoError(t, err)

	// kicks off the gc goroutine
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	shardKey := shard.KeyFromString(dah.String())

	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// doesn't exist yet
	assert.NotContains(t, edsStore.lastGCResult.Load().Shards, shardKey)

	// wait for gc to run, retry three times
	for i := 0; i < 3; i++ {
		time.Sleep(edsStore.gcInterval)
		if _, ok := edsStore.lastGCResult.Load().Shards[shardKey]; ok {
			break
		}
	}
	assert.Contains(t, edsStore.lastGCResult.Load().Shards, shardKey)

	// assert nil in this context means there was no error re-acquiring the shard during GC
	assert.Nil(t, edsStore.lastGCResult.Load().Shards[shardKey])
}

func Test_BlockstoreCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// key isnt in cache yet, so get returns errCacheMiss
	shardKey := shard.KeyFromString(dah.String())
	_, err = edsStore.cache.Get(shardKey)
	assert.ErrorIs(t, err, errCacheMiss)

	// now get it, so that the key is in the cache
	_, err = edsStore.getCachedAccessor(ctx, shardKey)
	assert.NoError(t, err)
	_, err = edsStore.cache.Get(shardKey)
	assert.NoError(t, err, errCacheMiss)
}

// Test_CachedAccessor verifies that the reader represented by a cached accessor can be read from
// multiple times, without exhausting the underlying reader.
func Test_CachedAccessor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	shardKey := shard.KeyFromString(dah.String())
	// adds to cache
	cachedAccessor, err := edsStore.getCachedAccessor(ctx, shardKey)
	assert.NoError(t, err)

	// first read
	carReader, err := car.NewCarReader(cachedAccessor.sa.Reader())
	assert.NoError(t, err)
	firstBlock, err := carReader.Next()
	assert.NoError(t, err)

	// second read
	cachedAccessor, err = edsStore.getCachedAccessor(ctx, shardKey)
	assert.NoError(t, err)
	carReader, err = car.NewCarReader(cachedAccessor.sa.Reader())
	assert.NoError(t, err)
	secondBlock, err := carReader.Next()
	assert.NoError(t, err)

	assert.Equal(t, firstBlock, secondBlock)
}

func BenchmarkStore(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	tmpDir := b.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := NewStore(tmpDir, ds)
	require.NoError(b, err)
	err = edsStore.Start(ctx)
	require.NoError(b, err)

	// BenchmarkStore/bench_put_128-10         	      10	3231859283 ns/op (~3sec)
	b.Run("bench put 128", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// pause the timer for initializing test data
			b.StopTimer()
			eds := edstest.RandEDS(b, 128)
			dah, err := da.NewDataAvailabilityHeader(eds)
			require.NoError(b, err)
			b.StartTimer()

			err = edsStore.Put(ctx, dah.Hash(), eds)
			require.NoError(b, err)
		}
	})

	// BenchmarkStore/bench_read_128-10         	      14	  78970661 ns/op (~70ms)
	b.Run("bench read 128", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// pause the timer for initializing test data
			b.StopTimer()
			eds := edstest.RandEDS(b, 128)
			dah, err := da.NewDataAvailabilityHeader(eds)
			require.NoError(b, err)
			_ = edsStore.Put(ctx, dah.Hash(), eds)
			b.StartTimer()

			_, err = edsStore.Get(ctx, dah.Hash())
			require.NoError(b, err)
		}
	})
}

func newStore(t *testing.T) (*Store, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return NewStore(tmpDir, ds)
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, share.Root) {
	eds := edstest.RandEDS(t, 4)
	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)

	return eds, dah
}
