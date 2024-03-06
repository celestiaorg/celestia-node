package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/celestiaorg/celestia-node/share/store/file"
)

const defaultCloseTimeout = time.Minute

var _ Cache = (*FileCache)(nil)

// FileCache implements the Cache interface using an LRU cache backend.
type FileCache struct {
	// The name is a prefix that will be used for cache metrics if they are enabled.
	name string
	// stripedLocks prevents simultaneous RW access to the file cache for an accessor. Instead
	// of using only one lock or one lock per key, we stripe the keys across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the key.
	stripedLocks [256]*sync.RWMutex
	// Caches the file for a given key for file read affinity, i.e., further reads will likely
	// be from the same file. Maps (Datahash -> accessor).
	cache *lru.Cache[key, *accessor]

	metrics *metrics
}

// accessor is the value stored in Cache. It implements the file.EdsFile interface. It has a
// reference counted so that it can be removed from the cache only when all references are released.
type accessor struct {
	file.EdsFile

	lock     sync.Mutex
	height   uint64
	done     chan struct{}
	refs     atomic.Int32
	isClosed bool
}

func NewFileCache(name string, cacheSize int) (*FileCache, error) {
	bc := &FileCache{
		name:         name,
		stripedLocks: [256]*sync.RWMutex{},
	}

	for i := range bc.stripedLocks {
		bc.stripedLocks[i] = &sync.RWMutex{}
	}
	// Instantiate the file Cache.
	bslru, err := lru.NewWithEvict[key, *accessor](cacheSize, bc.evictFn())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate accessor cache: %w", err)
	}
	bc.cache = bslru
	return bc, nil
}

// evictFn will be invoked when an item is evicted from the cache.
func (bc *FileCache) evictFn() func(key, *accessor) {
	return func(_ key, ac *accessor) {
		// we can release accessor from cache early, while it is being closed in parallel routine
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

// Get retrieves the accessor for a given key from the Cache. If the Accessor is not in
// the Cache, it returns an ErrCacheMiss.
func (bc *FileCache) Get(key key) (file.EdsFile, error) {
	lk := bc.getLock(key)
	lk.RLock()
	defer lk.RUnlock()

	ac, ok := bc.cache.Get(key)
	if !ok {
		bc.metrics.observeGet(false)
		return nil, ErrCacheMiss
	}

	bc.metrics.observeGet(true)
	return newRefCloser(ac)
}

// GetOrLoad attempts to get an item from the cache, and if not found, invokes
// the provided loader function to load it.
func (bc *FileCache) GetOrLoad(ctx context.Context, key key, loader OpenFileFn) (file.EdsFile, error) {
	lk := bc.getLock(key)
	lk.Lock()
	defer lk.Unlock()

	ac, ok := bc.cache.Get(key)
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

	ac = &accessor{EdsFile: f}
	// Create a new accessor first to increment the reference count in it, so it cannot get evicted
	// from the inner lru cache before it is used.
	rc, err := newRefCloser(ac)
	if err != nil {
		return nil, err
	}
	bc.cache.Add(key, ac)
	return rc, nil
}

// Remove removes the Accessor for a given key from the cache.
func (bc *FileCache) Remove(key key) error {
	lk := bc.getLock(key)
	lk.RLock()
	ac, ok := bc.cache.Get(key)
	lk.RUnlock()
	if !ok {
		// item is not in cache
		return nil
	}
	if err := ac.close(); err != nil {
		return err
	}
	// The cache will call evictFn on removal, where accessor close will be called.
	bc.cache.Remove(key)
	return nil
}

// EnableMetrics enables metrics for the cache.
func (bc *FileCache) EnableMetrics() error {
	var err error
	bc.metrics, err = newMetrics(bc)
	return err
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

	select {
	case <-done:
	case <-time.After(defaultCloseTimeout):
		return fmt.Errorf("closing file, some readers didn't close the file within timeout,"+
			" amount left: %v", s.refs.Load())
	}
	if err := s.EdsFile.Close(); err != nil {
		return fmt.Errorf("closing accessor: %w", err)
	}
	return nil
}

// refCloser manages references to accessor from provided reader and removes the ref, when the
// Close is called
type refCloser struct {
	*accessor
	closeFn func()
}

// newRefCloser creates new refCloser
func newRefCloser(abs *accessor) (*refCloser, error) {
	if err := abs.addRef(); err != nil {
		return nil, err
	}

	var closeOnce sync.Once
	return &refCloser{
		accessor: abs,
		closeFn: func() {
			closeOnce.Do(abs.removeRef)
		},
	}, nil
}

func (c *refCloser) Close() error {
	c.closeFn()
	return nil
}

func (bc *FileCache) getLock(k key) *sync.RWMutex {
	return bc.stripedLocks[byte(k%256)]
}
