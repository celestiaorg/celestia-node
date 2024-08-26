package store

import (
	"context"
	"fmt"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
	"github.com/celestiaorg/celestia-node/store/cache"
)

// CachedStore is a store with an additional cache layer. New cache layer is created on top of the
// original store cache. Parent store cache will be able to read from the new cache layer, but will
// not be able to write to it. Making parent store cache and CachedStore cache independent for writes.
type CachedStore struct {
	store         *Store
	combinedCache *cache.DoubleCache
}

// WithCache wraps store with extra layer of cache. Created caching layer will have read access to original
// store cache and will duplicate it's content. It updates parent store cache, to allow it to
// read from additionally created cache layer.
func (s *Store) WithCache(name string, size int) (*CachedStore, error) {
	newCache, err := cache.NewAccessorCache(name, size)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s cache: %w", name, err)
	}

	wrappedCache := cache.NewDoubleCache(s.cache, newCache)
	err = s.metrics.addCacheMetrics(wrappedCache)
	if err != nil {
		return nil, fmt.Errorf("failed to add cache metrics: %w", err)
	}
	// update parent store cache to allow it to read from both caches
	s.cache = wrappedCache
	return &CachedStore{
		store:         s,
		combinedCache: wrappedCache,
	}, nil
}

// GetByHeight returns accessor for given height and puts it into cache.
func (cs *CachedStore) GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error) {
	acc, err := cs.combinedCache.First().Get(height)
	if err == nil {
		return acc, nil
	}
	return cs.combinedCache.Second().GetOrLoad(ctx, height, cs.openFile(height))
}

func (cs *CachedStore) openFile(height uint64) cache.OpenAccessorFn {
	return func(context.Context) (eds.AccessorStreamer, error) {
		// open file directly wihout calling GetByHeight of inner getter to
		// avoid hitting store cache second time
		path := cs.store.heightToPath(height, "")
		return cs.store.openAccessor(path)
	}
}
