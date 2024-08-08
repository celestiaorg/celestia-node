package cache

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

const defaultCloseTimeout = time.Minute

var _ Cache = (*AccessorCache)(nil)

// AccessorCache implements the Cache interface using an LRU cache backend.
type AccessorCache struct {
	// The name is a prefix that will be used for cache metrics if they are enabled.
	name string
	// stripedLocks prevents simultaneous RW access to the accessor cache. Instead
	// of using only one lock or one lock per uint64, we stripe the uint64s across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the uint64.
	stripedLocks [256]*sync.RWMutex
	// Caches the accessor for a given uint64 for accessor read affinity, i.e., further reads will
	// likely be from the same accessor. Maps (Datahash -> accessor).
	cache *lru.Cache[uint64, *accessor]

	metrics *metrics
}

// accessor is the value stored in Cache. It implements the eds.AccessorStreamer interface. It has a
// reference counted so that it can be removed from the cache only when all references are released.
type accessor struct {
	eds.AccessorStreamer

	lock     sync.Mutex
	done     chan struct{}
	refs     atomic.Int32
	isClosed bool
}

func NewAccessorCache(name string, cacheSize int) (*AccessorCache, error) {
	bc := &AccessorCache{
		name:         name,
		stripedLocks: [256]*sync.RWMutex{},
	}

	for i := range bc.stripedLocks {
		bc.stripedLocks[i] = &sync.RWMutex{}
	}
	// Instantiate the Accessor Cache.
	bslru, err := lru.NewWithEvict[uint64, *accessor](cacheSize, bc.evictFn())
	if err != nil {
		return nil, fmt.Errorf("creating accessor cache %s: %w", name, err)
	}
	bc.cache = bslru
	return bc, nil
}

// evictFn will be invoked when an item is evicted from the cache.
func (bc *AccessorCache) evictFn() func(uint64, *accessor) {
	return func(_ uint64, ac *accessor) {
		// we don't want to block cache on close and can release accessor from cache early, while it is
		// being closed in parallel routine
		go func() {
			err := ac.close()
			if err != nil {
				bc.metrics.observeEvicted(true)
				log.Errorf("couldn't close accessor after cache eviction: %s", err)
				return
			}
			bc.metrics.observeEvicted(false)
		}()
	}
}

// Get retrieves the accessor for a given uint64 from the Cache. If the Accessor is not in
// the Cache, it returns an ErrCacheMiss.
func (bc *AccessorCache) Get(height uint64) (eds.AccessorStreamer, error) {
	lk := bc.getLock(height)
	lk.RLock()
	defer lk.RUnlock()

	ac, ok := bc.cache.Get(height)
	if !ok {
		bc.metrics.observeGet(false)
		return nil, ErrCacheMiss
	}

	bc.metrics.observeGet(true)
	return newRefCloser(ac)
}

// GetOrLoad attempts to get an item from the cache, and if not found, invokes
// the provided loader function to load it.
func (bc *AccessorCache) GetOrLoad(
	ctx context.Context,
	height uint64,
	loader OpenAccessorFn,
) (eds.AccessorStreamer, error) {
	lk := bc.getLock(height)
	lk.Lock()
	defer lk.Unlock()

	ac, ok := bc.cache.Get(height)
	if ok {
		// return accessor, only if it is not closed yet
		accessorWithRef, err := newRefCloser(ac)
		if err == nil {
			bc.metrics.observeGet(true)
			return accessorWithRef, nil
		}
	}

	// accessor not found in cache or closed, so load new one using loader
	f, err := loader(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load accessor: %w", err)
	}

	ac = &accessor{AccessorStreamer: f}
	// Create a new accessor first to increment the reference count in it, so it cannot get evicted
	// from the inner lru cache before it is used.
	rc, err := newRefCloser(ac)
	if err != nil {
		return nil, err
	}
	bc.cache.Add(height, ac)
	return rc, nil
}

// Remove removes the Accessor for a given uint64 from the cache.
func (bc *AccessorCache) Remove(height uint64) error {
	lk := bc.getLock(height)
	lk.RLock()
	ac, ok := bc.cache.Get(height)
	lk.RUnlock()
	if !ok {
		// item is not in cache
		return nil
	}
	if err := ac.close(); err != nil {
		return err
	}
	// The cache will call evictFn on removal, where accessor close will be called.
	bc.cache.Remove(height)
	return nil
}

// EnableMetrics enables metrics for the cache.
func (bc *AccessorCache) EnableMetrics() (unreg func() error, err error) {
	if bc.metrics == nil {
		bc.metrics, err = newMetrics(bc)
	}
	return bc.metrics.reg.Unregister, err
}

func (s *accessor) addRef() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.isClosed {
		// item is already closed and soon will be removed after all refs are released
		return ErrCacheMiss
	}
	if s.refs.Add(1) == 1 {
		// there were no refs previously and done channel was closed, reopen it by recreating
		s.done = make(chan struct{})
	}
	return nil
}

func (s *accessor) removeRef() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.refs.Add(-1) <= 0 {
		close(s.done)
	}
}

// close closes the accessor and removes it from the cache if it is not closed yet. It will block
// until all references are released or timeout is reached.
func (s *accessor) close() error {
	s.lock.Lock()
	if s.isClosed {
		s.lock.Unlock()
		// accessor will be closed by another goroutine
		return nil
	}
	s.isClosed = true
	done := s.done
	s.lock.Unlock()

	// wait until all references are released or timeout is reached. If timeout is reached, log an
	// error and close the accessor forcefully.
	select {
	case <-done:
	case <-time.After(defaultCloseTimeout):
		log.Errorf("closing accessor, some readers didn't close the accessor within timeout,"+
			" amount left: %v", s.refs.Load())
	}
	if err := s.AccessorStreamer.Close(); err != nil {
		return fmt.Errorf("closing accessor: %w", err)
	}
	return nil
}

// refCloser exists for reference counting protection on accessor. It ensures that a caller can't
// decrement it more than once.
type refCloser struct {
	*accessor
	closed    atomic.Bool
	removeRef func()
}

// newRefCloser creates new refCloser
func newRefCloser(abs *accessor) (*refCloser, error) {
	if err := abs.addRef(); err != nil {
		return nil, err
	}

	rf := &refCloser{
		accessor:  abs,
		removeRef: abs.removeRef,
	}
	// Set finalizer to ensure that accessor is closed when refCloser is garbage collected.
	// We expect that refCloser will be closed explicitly by the caller. If it is not closed,
	// we log an error.
	runtime.SetFinalizer(rf, func(rf *refCloser) {
		if rf.close() {
			log.Errorf("refCloser for accessor was garbage collected before Close was called")
		}
	})
	return rf, nil
}

func (c *refCloser) close() bool {
	if c.closed.CompareAndSwap(false, true) {
		c.removeRef()
		return true
	}
	return false
}

func (c *refCloser) Close() error {
	c.close()
	return nil
}

func (bc *AccessorCache) getLock(k uint64) *sync.RWMutex {
	return bc.stripedLocks[byte(k%256)]
}
