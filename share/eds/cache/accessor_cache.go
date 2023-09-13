package cache

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	lru "github.com/hashicorp/golang-lru/v2"
)

var _ Cache = (*AccessorCache)(nil)

// AccessorCache implements the Cache interface using an LRU cache backend.
type AccessorCache struct {
	// The name is a prefix that will be used for cache metrics if they are enabled.
	name string
	// stripedLocks prevents simultaneous RW access to the blockstore cache for a shard. Instead
	// of using only one lock or one lock per key, we stripe the shard keys across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the key.
	stripedLocks [256]sync.Mutex
	// Caches the blockstore for a given shard for shard read affinity, i.e., further reads will likely
	// be from the same shard. Maps (shard key -> blockstore).
	cache *lru.Cache[shard.Key, *accessorWithBlockstore]

	metrics *metrics
}

// accessorWithBlockstore is the value that we store in the blockstore Cache. It implements the
// Accessor interface.
type accessorWithBlockstore struct {
	sync.RWMutex
	shardAccessor Accessor
	// The blockstore is stored separately because each access to the blockstore over the shard
	// accessor reopens the underlying CAR.
	bs dagstore.ReadBlockstore
}

func NewAccessorCache(name string, cacheSize int) (*AccessorCache, error) {
	bc := &AccessorCache{
		name: name,
	}
	// Instantiate the blockstore Cache.
	bslru, err := lru.NewWithEvict[shard.Key, *accessorWithBlockstore](cacheSize, bc.evictFn())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate blockstore cache: %w", err)
	}
	bc.cache = bslru
	return bc, nil
}

// evictFn will be invoked when an item is evicted from the cache.
func (bc *AccessorCache) evictFn() func(shard.Key, *accessorWithBlockstore) {
	return func(_ shard.Key, abs *accessorWithBlockstore) {
		err := abs.shardAccessor.Close()
		if err != nil {
			bc.metrics.observeEvicted(true)
			log.Errorf("couldn't close accessor after cache eviction: %s", err)
			return
		}
		bc.metrics.observeEvicted(false)
	}
}

// Get retrieves the Accessor for a given shard key from the Cache. If the Accessor is not in
// the Cache, it returns an ErrCacheMiss.
func (bc *AccessorCache) Get(key shard.Key) (Accessor, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	accessor, err := bc.get(key)
	if err != nil {
		bc.metrics.observeGet(false)
		return nil, err
	}
	bc.metrics.observeGet(true)
	return newCloser(accessor), nil
}

func (bc *AccessorCache) get(key shard.Key) (*accessorWithBlockstore, error) {
	abs, ok := bc.cache.Get(key)
	if !ok {
		return nil, ErrCacheMiss
	}
	return abs, nil
}

// GetOrLoad attempts to get an item from the cache, and if not found, invokes
// the provided loader function to load it.
func (bc *AccessorCache) GetOrLoad(
	ctx context.Context,
	key shard.Key,
	loader func(context.Context, shard.Key) (Accessor, error),
) (Accessor, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	abs, err := bc.get(key)
	if err == nil {
		bc.metrics.observeGet(true)
		return newCloser(abs), nil
	}

	// accessor not found in cache, so load new one using loader
	accessor, err := loader(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("unable to load accessor: %w", err)
	}

	abs = &accessorWithBlockstore{
		shardAccessor: accessor,
	}

	// Create a new accessor first to increment the reference count in it, so it cannot get evicted
	// from the inner lru cache before it is used.
	ac := newCloser(abs)
	bc.cache.Add(key, abs)
	return ac, nil
}

// Remove removes the Accessor for a given key from the cache.
func (bc *AccessorCache) Remove(key shard.Key) error {
	// The cache will call evictFn on removal, where accessor close will be called.
	bc.cache.Remove(key)
	return nil
}

// EnableMetrics enables metrics for the cache.
func (bc *AccessorCache) EnableMetrics() error {
	var err error
	bc.metrics, err = newMetrics(bc)
	return err
}

// Blockstore implements the Blockstore of the Accessor interface. It creates the blockstore on the
// first request and reuses the created instance for all subsequent requests.
func (s *accessorWithBlockstore) Blockstore() (dagstore.ReadBlockstore, error) {
	s.Lock()
	defer s.Unlock()
	var err error
	if s.bs == nil {
		s.bs, err = s.shardAccessor.Blockstore()
	}
	return s.bs, err
}

// Reader returns a new copy of the reader to read data.
func (s *accessorWithBlockstore) Reader() io.Reader {
	return s.shardAccessor.Reader()
}

// accessorCloser is a temporary object before reference counting is implemented.
type accessorCloser struct {
	*accessorWithBlockstore
	io.Closer
}

func newCloser(abs *accessorWithBlockstore) *accessorCloser {
	return &accessorCloser{
		accessorWithBlockstore: abs,
		Closer:                 io.NopCloser(nil),
	}
}

// shardKeyToStriped returns the index of the lock to use for a given shard key. We use the last
// byte of the shard key as the pseudo-random index.
func shardKeyToStriped(sk shard.Key) byte {
	return sk.String()[len(sk.String())-1]
}
