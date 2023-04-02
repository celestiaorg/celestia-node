package eds

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	lru "github.com/hashicorp/golang-lru"
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
}

func newBlockstoreCache(cacheSize int) (*blockstoreCache, error) {
	// instantiate the blockstore cache
	bslru, err := lru.NewWithEvict(cacheSize, func(_ interface{}, val interface{}) {
		// ensure we close the blockstore for a shard when it's evicted so dagstore can gc it.
		abs, ok := val.(*accessorWithBlockstore)
		if !ok {
			panic(fmt.Sprintf(
				"casting value from cache to accessorWithBlockstore: %s",
				reflect.TypeOf(val),
			))
		}

		if err := abs.sa.Close(); err != nil {
			log.Errorf("couldn't close accessor after cache eviction: %s", err)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate blockstore cache: %w", err)
	}
	return &blockstoreCache{cache: bslru}, nil
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
