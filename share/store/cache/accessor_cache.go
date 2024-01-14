package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/store/file"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

const defaultCloseTimeout = time.Minute

var _ Cache = (*FileCache)(nil)

// FileCache implements the Cache interface using an LRU cache backend.
type FileCache struct {
	idxLock sync.RWMutex
	// The name is a prefix that will be used for cache metrics if they are enabled.
	name string
	// HeightsIdx is a map of Height to Datahash. It is used to find the Datahash for a given Height.
	HeightsIdx map[uint64]string
	// stripedLocks prevents simultaneous RW access to the file cache for an accessor. Instead
	// of using only one lock or one lock per key, we stripe the keys across 256 locks. 256 is
	// chosen because it 0-255 is the range of values we get looking at the last byte of the key.
	stripedLocks [256]sync.Mutex
	// Caches the file for a given key for file read affinity, i.e., further reads will likely
	// be from the same file. Maps (Datahash -> accessor).
	cache *lru.Cache[string, *accessor]

	metrics *metrics
}

// accessor is the value stored in Cache. It implements the file.EdsFile interface. It has a
// reference counted so that it can be removed from the cache only when all references are released.
type accessor struct {
	lock sync.RWMutex
	file.EdsFile

	Height   uint64
	done     chan struct{}
	refs     atomic.Int32
	isClosed bool
}

func (s *accessor) addRef() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.isClosed {
		// item is already closed and soon will be removed after all refs are released
		return errCacheMiss
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

func NewFileCache(name string, cacheSize int) (*FileCache, error) {
	bc := &FileCache{
		name: name,
	}
	// Instantiate the file Cache.
	bslru, err := lru.NewWithEvict[string, *accessor](cacheSize, bc.evictFn())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate accessor cache: %w", err)
	}
	bc.cache = bslru
	return bc, nil
}

// evictFn will be invoked when an item is evicted from the cache.
func (bc *FileCache) evictFn() func(string, *accessor) {
	return func(_ string, fa *accessor) {
		bc.idxLock.Lock()
		defer bc.idxLock.Unlock()
		delete(bc.HeightsIdx, fa.Height)
		// we can release accessor from cache early, while it is being closed in parallel routine
		go func() {
			err := fa.close()
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
// the Cache, it returns an errCacheMiss.
func (bc *FileCache) Get(key Key) (file.EdsFile, error) {
	lk := &bc.stripedLocks[keyToStriped(key)]
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

func (bc *FileCache) get(key Key) (*accessor, error) {
	hashStr := key.Datahash.String()
	if hashStr == "" {
		var ok bool
		bc.idxLock.RLock()
		hashStr, ok = bc.HeightsIdx[key.Height]
		bc.idxLock.RUnlock()
		if !ok {
			return nil, errCacheMiss
		}
	}
	abs, ok := bc.cache.Get(hashStr)
	if !ok {
		return nil, errCacheMiss
	}
	return abs, nil
}

// GetOrLoad attempts to get an item from the cache, and if not found, invokes
// the provided loader function to load it.
func (bc *FileCache) GetOrLoad(ctx context.Context, key Key, loader OpenFileFn) (file.EdsFile, error) {
	if !key.isComplete() {
		return nil, errors.New("key is not complete")
	}

	lk := &bc.stripedLocks[keyToStriped(key)]
	lk.Lock()
	defer lk.Unlock()

	abs, err := bc.get(key)
	if err == nil {
		// return accessor, only if it is not closed yet
		accessorWithRef, err := newRefCloser(abs)
		if err == nil {
			bc.metrics.observeGet(true)
			return accessorWithRef, nil
		}
	}

	// accessor not found in cache or closed, so load new one using loader
	key, file, err := loader(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load accessor: %w", err)
	}

	fa := &accessor{EdsFile: file}
	// Create a new accessor first to increment the reference count in it, so it cannot get evicted
	// from the inner lru cache before it is used.
	rc, err := newRefCloser(fa)
	if err != nil {
		return nil, err
	}
	return rc, bc.add(key, fa)
}

func (bc *FileCache) add(key Key, fa *accessor) error {
	keyStr := key.Datahash.String()
	bc.idxLock.Lock()
	defer bc.idxLock.Unlock()
	// Create a new accessor first to increment the reference count in it, so it cannot get evicted
	// from the inner lru cache before it is used.
	bc.cache.Add(keyStr, fa)
	bc.HeightsIdx[key.Height] = keyStr
	return nil
}

// Remove removes the Accessor for a given key from the cache.
func (bc *FileCache) Remove(key Key) error {
	lk := &bc.stripedLocks[keyToStriped(key)]
	lk.Lock()
	accessor, err := bc.get(key)
	lk.Unlock()
	if errors.Is(err, errCacheMiss) {
		// item is not in cache
		return nil
	}
	if err = accessor.close(); err != nil {
		return err
	}
	// The cache will call evictFn on removal, where accessor close will be called.
	bc.cache.Remove(key.Datahash.String())
	return nil
}

// EnableMetrics enables metrics for the cache.
func (bc *FileCache) EnableMetrics() error {
	var err error
	bc.metrics, err = newMetrics(bc)
	return err
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

// keyToStriped returns the index of the lock to use for a given key. We use the last
// byte of the Datahash as the pseudo-random index.
func keyToStriped(sk Key) byte {
	str := sk.Datahash.String()
	return str[len(str)-1]
}
