package eds

import (
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-car"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	dsbadger "github.com/celestiaorg/go-ds-badger4"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/cache"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
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
		defer func() {
			require.NoError(t, r.Close())
		}()
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

		// accessor will be registered in cache async on put, so give it some time to settle
		time.Sleep(time.Millisecond * 100)

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

	t.Run("Remove after OpShardFail", func(t *testing.T) {
		eds, dah := randomEDS(t)

		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		// assert that shard now exists
		ok, err := edsStore.Has(ctx, dah.Hash())
		assert.NoError(t, err)
		assert.True(t, ok)

		// assert that file now exists
		path := edsStore.basepath + blocksPath + dah.String()
		_, err = os.Stat(path)
		assert.NoError(t, err)

		err = os.Remove(path)
		assert.NoError(t, err)

		// accessor will be registered in cache async on put, so give it some time to settle
		time.Sleep(time.Millisecond * 100)

		// remove non-failed accessor from cache
		err = edsStore.cache.Remove(shard.KeyFromString(dah.String()))
		assert.NoError(t, err)

		_, err = edsStore.GetCAR(ctx, dah.Hash())
		assert.Error(t, err)

		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				has, err := edsStore.Has(ctx, dah.Hash())
				if err == nil && !has {
					// shard no longer exists after OpShardFail was detected from GetCAR call
					return
				}
			case <-ctx.Done():
				t.Fatal("timeout waiting for shard to be removed")
			}
		}
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

	t.Run("RecentBlocksCache", func(t *testing.T) {
		eds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		// accessor will be registered in cache async on put, so give it some time to settle
		time.Sleep(time.Millisecond * 100)

		// check, that the key is in the cache after put
		shardKey := shard.KeyFromString(dah.String())
		_, err = edsStore.cache.Get(shardKey)
		assert.NoError(t, err)
	})

	t.Run("List", func(t *testing.T) {
		const amount = 10
		hashes := make([]share.DataHash, 0, amount)
		for range make([]byte, amount) {
			eds, dah := randomEDS(t)
			err = edsStore.Put(ctx, dah.Hash(), eds)
			require.NoError(t, err)
			hashes = append(hashes, dah.Hash())
		}

		hashesOut, err := edsStore.List()
		require.NoError(t, err)
		for _, hash := range hashes {
			assert.Contains(t, hashesOut, hash)
		}
	})

	t.Run("Parallel put", func(t *testing.T) {
		const amount = 20
		eds, dah := randomEDS(t)

		wg := sync.WaitGroup{}
		for i := 1; i < amount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := edsStore.Put(ctx, dah.Hash(), eds)
				if err != nil {
					require.ErrorIs(t, err, dagstore.ErrShardExists)
				}
			}()
		}
		wg.Wait()

		eds, err := edsStore.Get(ctx, dah.Hash())
		require.NoError(t, err)
		newDah, err := da.NewDataAvailabilityHeader(eds)
		require.NoError(t, err)
		require.Equal(t, dah.Hash(), newDah.Hash())
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

	// accessor will be registered in cache async on put, so give it some time to settle
	time.Sleep(time.Millisecond * 100)

	// remove links to the shard from cache
	time.Sleep(time.Millisecond * 100)
	key := shard.KeyFromString(share.DataHash(dah.Hash()).String())
	err = edsStore.cache.Remove(key)
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

	// store eds to the store with noopCache to allow clean cache after put
	swap := edsStore.cache
	edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})
	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// get any key from saved eds
	bs, err := edsStore.carBlockstore(ctx, dah.Hash())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bs.Close())
	}()
	keys, err := bs.AllKeysChan(ctx)
	require.NoError(t, err)
	var key cid.Cid
	select {
	case key = <-keys:
	case <-ctx.Done():
		t.Fatal("context timeout")
	}

	// swap back original cache
	edsStore.cache = swap

	// key shouldn't be in cache yet, check for returned errCacheMiss
	shardKey := shard.KeyFromString(dah.String())
	_, err = edsStore.cache.Get(shardKey)
	require.Error(t, err)

	// now get it from blockstore, to trigger storing to cache
	_, err = edsStore.Blockstore().Get(ctx, key)
	require.NoError(t, err)

	// should be no errCacheMiss anymore
	_, err = edsStore.cache.Get(shardKey)
	require.NoError(t, err)
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

	// accessor will be registered in cache async on put, so give it some time to settle
	time.Sleep(time.Millisecond * 100)

	// accessor should be in cache
	_, err = edsStore.cache.Get(shard.KeyFromString(dah.String()))
	require.NoError(t, err)

	// first read from cached accessor
	carReader, err := edsStore.getCAR(ctx, dah.Hash())
	require.NoError(t, err)
	firstBlock, err := io.ReadAll(carReader)
	require.NoError(t, err)
	require.NoError(t, carReader.Close())

	// second read from cached accessor
	carReader, err = edsStore.getCAR(ctx, dah.Hash())
	require.NoError(t, err)
	secondBlock, err := io.ReadAll(carReader)
	require.NoError(t, err)
	require.NoError(t, carReader.Close())

	require.Equal(t, firstBlock, secondBlock)
}

// Test_CachedAccessor verifies that the reader represented by a accessor obtained directly from
// dagstore can be read from multiple times, without exhausting the underlying reader.
func Test_NotCachedAccessor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)
	// replace cache with noopCache to
	edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})

	eds, dah := randomEDS(t)
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	// accessor will be registered in cache async on put, so give it some time to settle
	time.Sleep(time.Millisecond * 100)

	// accessor should not be in cache
	_, err = edsStore.cache.Get(shard.KeyFromString(dah.String()))
	require.Error(t, err)

	// first read from direct accessor (not from cache)
	carReader, err := edsStore.getCAR(ctx, dah.Hash())
	require.NoError(t, err)
	firstBlock, err := io.ReadAll(carReader)
	require.NoError(t, err)
	require.NoError(t, carReader.Close())

	// second read from direct accessor (not from cache)
	carReader, err = edsStore.getCAR(ctx, dah.Hash())
	require.NoError(t, err)
	secondBlock, err := io.ReadAll(carReader)
	require.NoError(t, err)
	require.NoError(t, carReader.Close())

	require.Equal(t, firstBlock, secondBlock)
}

func BenchmarkStore(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := NewStore(DefaultParameters(), b.TempDir(), ds)
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
			dah, err := share.NewRoot(eds)
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
			dah, err := share.NewRoot(eds)
			require.NoError(b, err)
			_ = edsStore.Put(ctx, dah.Hash(), eds)
			b.StartTimer()

			_, err = edsStore.Get(ctx, dah.Hash())
			require.NoError(b, err)
		}
	})
}

// BenchmarkCacheEviction benchmarks the time it takes to load a block to the cache, when the
// cache size is set to 1. This forces cache eviction on every read.
// BenchmarkCacheEviction-10/128    	     384	   3533586 ns/op (~3ms)
func BenchmarkCacheEviction(b *testing.B) {
	const (
		blocks = 4
		size   = 128
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	b.Cleanup(cancel)

	dir := b.TempDir()
	ds, err := dsbadger.NewDatastore(dir, &dsbadger.DefaultOptions)
	require.NoError(b, err)

	newStore := func(params *Parameters) *Store {
		edsStore, err := NewStore(params, dir, ds)
		require.NoError(b, err)
		err = edsStore.Start(ctx)
		require.NoError(b, err)
		return edsStore
	}
	edsStore := newStore(DefaultParameters())

	// generate EDSs and store them
	cids := make([]cid.Cid, blocks)
	for i := range cids {
		eds := edstest.RandEDS(b, size)
		dah, err := da.NewDataAvailabilityHeader(eds)
		require.NoError(b, err)
		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(b, err)

		// store cids for read loop later
		cids[i] = ipld.MustCidFromNamespacedSha256(dah.RowRoots[0])
	}

	// restart store to clear cache
	require.NoError(b, edsStore.Stop(ctx))

	// set BlockstoreCacheSize to 1 to force eviction on every read
	params := DefaultParameters()
	params.BlockstoreCacheSize = 1
	bstore := newStore(params).Blockstore()

	// start benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := cids[i%blocks]
		// every read will trigger eviction
		_, err := bstore.Get(ctx, h)
		require.NoError(b, err)
	}
}

func newStore(t *testing.T) (*Store, error) {
	t.Helper()

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return NewStore(DefaultParameters(), t.TempDir(), ds)
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.Root) {
	eds := edstest.RandEDS(t, 4)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)

	return eds, dah
}
