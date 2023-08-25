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
	errCacheMiss = errors.New("accessor not found in blockstore cache")
)

// cache is an interface that defines the basic cache operations.
type cache interface {
	// Get retrieves an item from the cache.
	get(shard.Key) (*accessorWithBlockstore, error)

	// getOrLoad attempts to get an item from the cache and, if not found, invokes
	// the provided loader function to load it into the cache.
	getOrLoad(ctx context.Context, key shard.Key, loader func(context.Context, shard.Key) (*dagstore.ShardAccessor, error)) (*accessorWithBlockstore, error)

	// enableMetrics enables metrics in cache
	enableMetrics() error
}

// multiCache represents a cache that looks into multiple caches one by one.
type multiCache struct {
	caches []cache
}

// newMultiCache creates a new multiCache with the provided caches.
func newMultiCache(caches ...cache) *multiCache {
	return &multiCache{caches: caches}
}

// get looks for an item in all the caches one by one and returns the first found item.
func (mc *multiCache) get(key shard.Key) (*accessorWithBlockstore, error) {
	for _, cache := range mc.caches {
		accessor, err := cache.get(key)
		if err == nil {
			return accessor, nil
		}
	}

	return nil, errCacheMiss
}

// getOrLoad attempts to get an item from all caches, and if not found, invokes
// the provided loader function to load it into one of the caches.
func (mc *multiCache) getOrLoad(
	ctx context.Context,
	key shard.Key,
	loader func(context.Context, shard.Key) (*dagstore.ShardAccessor, error),
) (*accessorWithBlockstore, error) {
	for _, cache := range mc.caches {
		accessor, err := cache.getOrLoad(ctx, key, loader)
		if err == nil {
			return accessor, nil
		}
	}

	return nil, errors.New("multicache: unable to get or load accessor")
}

func (mc *multiCache) enableMetrics() error {
	for _, cache := range mc.caches {
		err := cache.enableMetrics()
		if err != nil {
			return err
		}
	}
	return nil
}

// accessorWithBlockstore is the value that we store in the blockstore cache
type accessorWithBlockstore struct {
	sa *dagstore.ShardAccessor
	// blockstore is stored separately because each access to the blockstore over the shard accessor
	// reopens the underlying CAR.
	bs dagstore.ReadBlockstore
}

type blockstoreCache struct {
	// name is a prefix, that will be used for cache metrics if it is enabled
	name string
	// stripedLocks prevents simultaneous RW access to the blockstore cache for a shard. Instead
	// of using only one lock or one lock per key, we stripe the shard keys across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the key.
	stripedLocks [256]sync.Mutex
	// caches the blockstore for a given shard for shard read affinity i.e.
	// further reads will likely be from the same shard. Maps (shard key -> blockstore).
	cache *lru.Cache

	metrics *cacheMetrics
}

func newBlockstoreCache(name string, cacheSize int) (*blockstoreCache, error) {
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

// Get retrieves the blockstore for a given shard key from the cache. If the blockstore is not in
// the cache, it returns an errCacheMiss
func (bc *blockstoreCache) get(key shard.Key) (*accessorWithBlockstore, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	return bc.unsafeGet(key)
}

func (bc *blockstoreCache) unsafeGet(key shard.Key) (*accessorWithBlockstore, error) {
	// We've already ensured that the given shard has the cid/multihash we are looking for.
	val, ok := bc.cache.Get(key)
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

// getOrLoad attempts to get an item from all caches, and if not found, invokes
// the provided loader function to load it into one of the caches.
func (bc *blockstoreCache) getOrLoad(
	ctx context.Context,
	key shard.Key,
	loader func(context.Context, shard.Key) (*dagstore.ShardAccessor, error),
) (*accessorWithBlockstore, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	if accessor, err := bc.unsafeGet(key); err == nil {
		return accessor, nil
	}

	accessor, err := loader(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("unable to get accessor: %w", err)
	}

	blockStore, err := accessor.Blockstore()
	if err != nil {
		return nil, fmt.Errorf("failed to get blockstore from accessor: %w", err)
	}

	newAccessor := &accessorWithBlockstore{
		bs: blockStore,
		sa: accessor,
	}
	bc.cache.Add(key, newAccessor)
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

func (bc *blockstoreCache) enableMetrics() error {
	evictedCounter, err := meter.Int64Counter("eds_blockstore_cache"+bc.name+"_evicted_counter",
		metric.WithDescription("eds blockstore cache evicted event counter"))
	if err != nil {
		return err
	}

	cacheSize, err := meter.Int64ObservableGauge("eds_blockstore"+bc.name+"_cache_size",
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

type noopCache struct{}

func (n noopCache) get(shard.Key) (*accessorWithBlockstore, error) {
	return nil, errCacheMiss
}

func (n noopCache) getOrLoad(
	context.Context, shard.Key,
	func(context.Context, shard.Key) (*dagstore.ShardAccessor, error),
) (*accessorWithBlockstore, error) {
	return nil, nil
}

func (n noopCache) enableMetrics() error {
	return nil
}
