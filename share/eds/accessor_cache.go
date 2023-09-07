package eds

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	lru "github.com/hashicorp/golang-lru"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	defaultCacheSize = 128
	errCacheMiss     = errors.New("accessor not found in blockstore cache")
)

// accessorWithBlockstore is the value that we store in the blockstore cache
type accessorWithBlockstore struct {
	sa *dagstore.ShardAccessor
	// blockstore is stored separately because each access to the blockstore over the shard accessor
	// reopens the underlying CAR.
	bs dagstore.ReadBlockstore
}

type blockstoreCache struct {
	// stripedLocks prevents simultaneous RW access to the blockstore cache for a shard. Instead
	// of using only one lock or one lock per key, we stripe the shard keys across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the key.
	stripedLocks [256]sync.Mutex
	// caches the blockstore for a given shard for shard read affinity i.e.
	// further reads will likely be from the same shard. Maps (shard key -> blockstore).
	cache *lru.Cache

	metrics *cacheMetrics
}

func newBlockstoreCache(cacheSize int) (*blockstoreCache, error) {
	bc := &blockstoreCache{}
	// instantiate the blockstore cache
	bslru, err := lru.NewWithEvict(cacheSize, bc.evictFn())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate blockstore cache: %w", err)
	}
	bc.cache = bslru
	return bc, nil
}

func (bc *blockstoreCache) evictFn() func(_ interface{}, val interface{}) {
	return func(_ interface{}, val interface{}) {
		// ensure we close the blockstore for a shard when it's evicted so dagstore can gc it.
		abs, ok := val.(*accessorWithBlockstore)
		if !ok {
			panic(fmt.Sprintf(
				"casting value from cache to accessorWithBlockstore: %s",
				reflect.TypeOf(val),
			))
		}

		err := abs.sa.Close()
		if err != nil {
			log.Errorf("couldn't close accessor after cache eviction: %s", err)
		}
		bc.metrics.observeEvicted(err != nil)
	}
}

func (bc *blockstoreCache) Remove(key shard.Key) bool {
	lk := &bc.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	return bc.cache.Remove(key)
}

// Get retrieves the blockstore for a given shard key from the cache. If the blockstore is not in
// the cache, it returns an errCacheMiss
func (bc *blockstoreCache) Get(shardContainingCid shard.Key) (*accessorWithBlockstore, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(shardContainingCid)]
	lk.Lock()
	defer lk.Unlock()

	return bc.unsafeGet(shardContainingCid)
}

func (bc *blockstoreCache) unsafeGet(shardContainingCid shard.Key) (*accessorWithBlockstore, error) {
	// We've already ensured that the given shard has the cid/multihash we are looking for.
	val, ok := bc.cache.Get(shardContainingCid)
	if !ok {
		return nil, errCacheMiss
	}

	accessor, ok := val.(*accessorWithBlockstore)
	if !ok {
		panic(fmt.Sprintf(
			"casting value from cache to accessorWithBlockstore: %s",
			reflect.TypeOf(val),
		))
	}
	return accessor, nil
}

// Add adds a blockstore for a given shard key to the cache.
func (bc *blockstoreCache) Add(
	shardContainingCid shard.Key,
	accessor *dagstore.ShardAccessor,
) (*accessorWithBlockstore, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(shardContainingCid)]
	lk.Lock()
	defer lk.Unlock()

	return bc.unsafeAdd(shardContainingCid, accessor)
}

func (bc *blockstoreCache) unsafeAdd(
	shardContainingCid shard.Key,
	accessor *dagstore.ShardAccessor,
) (*accessorWithBlockstore, error) {
	blockStore, err := accessor.Blockstore()
	if err != nil {
		return nil, fmt.Errorf("failed to get blockstore from accessor: %w", err)
	}

	newAccessor := &accessorWithBlockstore{
		bs: blockStore,
		sa: accessor,
	}
	bc.cache.Add(shardContainingCid, newAccessor)
	return newAccessor, nil
}

// shardKeyToStriped returns the index of the lock to use for a given shard key. We use the last
// byte of the shard key as the pseudo-random index.
func shardKeyToStriped(sk shard.Key) byte {
	return sk.String()[len(sk.String())-1]
}

type cacheMetrics struct {
	evictedCounter metric.Int64Counter
}

func (bc *blockstoreCache) withMetrics() error {
	evictedCounter, err := meter.Int64Counter("eds_blockstore_cache_evicted_counter",
		metric.WithDescription("eds blockstore cache evicted event counter"))
	if err != nil {
		return err
	}

	cacheSize, err := meter.Int64ObservableGauge("eds_blockstore_cache_size",
		metric.WithDescription("total amount of items in blockstore cache"),
	)
	if err != nil {
		return err
	}

	callback := func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(cacheSize, int64(bc.cache.Len()))
		return nil
	}
	_, err = meter.RegisterCallback(callback, cacheSize)
	if err != nil {
		return err
	}
	bc.metrics = &cacheMetrics{evictedCounter: evictedCounter}
	return nil
}

func (m *cacheMetrics) observeEvicted(failed bool) {
	if m == nil {
		return
	}
	m.evictedCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.Bool(failedKey, failed)))
}
