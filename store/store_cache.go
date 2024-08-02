package store

import (
	"context"
	"fmt"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/store/cache"
)

// CachedStore wraps Store with cache, that put items into cache on every GetByHeight
// call. It updates parent Store cache, to allow it to read from additionally created cache.
type CachedStore struct {
	inner *Store
	cache *cache.DoubleCache
}

// WithCache creates new CachedStore with cache of the specified size.
func (s *Store) WithCache(name string, size int) (*CachedStore, error) {
	newCache, err := cache.NewAccessorCache(name, size)
	if err != nil {
		return nil, fmt.Errorf("failed to create availability cache: %w", err)
	}

	wrappedCache := cache.NewDoubleCache(s.cache, newCache)
	s.metrics.addCacheMetrics(wrappedCache)
	s.cache = wrappedCache
	return &CachedStore{
		inner: s,
		cache: wrappedCache,
	}, nil
}

// GetByHeight returns accessor for given height and puts it into cache.
func (s *CachedStore) GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error) {
	acc, err := s.cache.First().Get(height)
	if err == nil {
		return acc, err
	}
	return s.cache.Second().GetOrLoad(ctx, height, s.openFile(height))
}

func (s *CachedStore) openFile(height uint64) cache.OpenAccessorFn {
	return func(ctx context.Context) (eds.AccessorStreamer, error) {
		// open file directly wihout calling GetByHeight of inner getter to
		// avoid hitting store cache second time
		path := s.inner.heightToPath(height)
		return s.inner.openFile(path)
	}
}
