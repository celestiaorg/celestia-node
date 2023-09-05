package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	lru "github.com/hashicorp/golang-lru"
)

const defaultCloseTimeout = time.Minute

var _ Cache = (*AccessorCache)(nil)

type AccessorCache struct {
	// name is a prefix, that will be used for cache metrics if it is enabled
	name string
	// stripedLocks prevents simultaneous RW access to the blockstore cache for a shard. Instead
	// of using only one lock or one lock per key, we stripe the shard keys across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the key.
	stripedLocks [256]sync.Mutex
	// caches the blockstore for a given shard for shard read affinity i.e.
	// further reads will likely be from the same shard. Maps (shard key -> blockstore).
	cache *lru.Cache

	metrics *metrics
}

// accessorWithBlockstore is the value that we store in the blockstore Cache. Implements Accessor
// interface
type accessorWithBlockstore struct {
	sync.RWMutex
	shardAccessor Accessor
	// blockstore is stored separately because each access to the blockstore over the shard accessor
	// reopens the underlying CAR.
	bs dagstore.ReadBlockstore

	done     chan struct{}
	refs     atomic.Int32
	isClosed bool
}

func NewAccessorCache(name string, cacheSize int) (*AccessorCache, error) {
	bc := &AccessorCache{
		name: name,
	}
	// instantiate the blockstore Cache
	bslru, err := lru.NewWithEvict(cacheSize, bc.evictFn())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate blockstore cache: %w", err)
	}
	bc.cache = bslru
	return bc, nil
}

func (bc *AccessorCache) evictFn() func(_ interface{}, val interface{}) {
	return func(_ interface{}, val interface{}) {
		// ensure we close the blockstore for a shard when it's evicted so dagstore can gc it.
		abs, ok := val.(*accessorWithBlockstore)
		if !ok {
			panic(fmt.Sprintf(
				"casting value from cache to accessorWithBlockstore: %s",
				reflect.TypeOf(val),
			))
		}

		// we can release accessor from cache early, while it is being closed in parallel routine
		go abs.close()
		bc.metrics.observeEvicted()
	}
}

// Get retrieves the blockstore for a given shard key from the Cache. If the blockstore is not in
// the Cache, it returns an errCacheMiss
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
	return newRefCloser(accessor)
}

func (bc *AccessorCache) get(key shard.Key) (*accessorWithBlockstore, error) {
	// We've already ensured that the given shard has the cid/multihash we are looking for.
	val, ok := bc.cache.Get(key)
	if !ok {
		return nil, ErrCacheMiss
	}

	abs, ok := val.(*accessorWithBlockstore)
	if !ok {
		panic(fmt.Sprintf(
			"casting value from cache to accessorWithBlockstore: %s",
			reflect.TypeOf(val),
		))
	}
	return abs, nil
}

// GetOrLoad attempts to get an item from all caches, and if not found, invokes
// the provided loader function to load it into one of the caches.
func (bc *AccessorCache) GetOrLoad(
	ctx context.Context,
	key shard.Key,
	loader func(context.Context, shard.Key) (Accessor, error),
) (Accessor, error) {
	lk := &bc.stripedLocks[shardKeyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	if accessor, err := bc.get(key); err == nil {
		return newRefCloser(accessor)
	}

	provider, err := loader(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("unable to get accessor: %w", err)
	}

	abs := &accessorWithBlockstore{
		shardAccessor: provider,
	}

	// create new accessor first to inc ref count in it, so it could not get evicted from inner cache
	// before it is used
	accessor, err := newRefCloser(abs)
	if err != nil {
		return nil, err
	}
	bc.cache.Add(key, abs)
	return accessor, nil
}

func (bc *AccessorCache) Remove(key shard.Key) error {
	accessor, err := bc.get(key)
	if errors.Is(err, ErrCacheMiss) {
		// item is not in cache
		return nil
	}
	if err = accessor.close(); err != nil {
		return err
	}
	// cache will call evictFn on removal, where accessor close will be called second time, but in
	// async manner
	bc.cache.Remove(key)
	return nil
}

func (bc *AccessorCache) EnableMetrics() error {
	var err error
	bc.metrics, err = newMetrics(bc)
	return err
}

// Blockstore implements Blockstore of the Accessor interface. It creates blockstore on first
// request and reuses created instance for all next requests.
func (s *accessorWithBlockstore) Blockstore() (dagstore.ReadBlockstore, error) {
	s.Lock()
	defer s.Unlock()
	var err error
	if s.bs == nil {
		s.bs, err = s.shardAccessor.Blockstore()
	}

	return s.bs, err
}

// Reader returns new copy of reader to read data
func (s *accessorWithBlockstore) Reader() io.Reader {
	return s.shardAccessor.Reader()
}

func (s *accessorWithBlockstore) addRef() error {
	s.RLock()
	defer s.RUnlock()
	if s.isClosed {
		// item is closed, so pretend it is a miss
		return ErrCacheMiss
	}
	if s.refs.Add(1) == 1 {
		// there were no refs previously and done channel was closed, reopen it by recreating
		s.done = make(chan struct{})
	}
	return nil
}

func (s *accessorWithBlockstore) removeRef() {
	s.RLock()
	defer s.RUnlock()
	if s.refs.Add(-1) <= 0 {
		close(s.done)
	}
}

func (s *accessorWithBlockstore) close() error {
	s.Lock()
	s.isClosed = true
	done := s.done
	s.Unlock()

	// TODO: add closing metrics
	select {
	case <-done:
	case <-time.After(defaultCloseTimeout):
		return fmt.Errorf("closing accessor, some readers didn't close the accessor within timeout,"+
			" amount left: %v", s.refs.Load())
	}
	if err := s.shardAccessor.Close(); err != nil {
		return fmt.Errorf("closing accessor: %w", err)
	}
	return nil
}

// refCloser manages references to accessor from provided reader and removes the ref, when the
// Close is called
type refCloser struct {
	*accessorWithBlockstore
	closeFn func()
}

// newRefCloser creates new refCloser
func newRefCloser(abs *accessorWithBlockstore) (*refCloser, error) {
	if err := abs.addRef(); err != nil {
		return nil, err
	}

	var closeOnce sync.Once
	return &refCloser{
		accessorWithBlockstore: abs,
		closeFn: func() {
			closeOnce.Do(abs.removeRef)
		},
	}, nil
}

func (c *refCloser) Close() error {
	c.closeFn()
	return nil
}

// shardKeyToStriped returns the index of the lock to use for a given shard key. We use the last
// byte of the shard key as the pseudo-random index.
func shardKeyToStriped(sk shard.Key) byte {
	return sk.String()[len(sk.String())-1]
}
