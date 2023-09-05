package cache

import (
	"context"
	"errors"

	"github.com/filecoin-project/dagstore/shard"
)

var _ Cache = (*MultiCache)(nil)

// MultiCache represents a Cache that looks into multiple caches one by one.
type MultiCache struct {
	first, second Cache
}

// NewMultiCache creates a new MultiCache with the provided caches.
func NewMultiCache(first, second Cache) *MultiCache {
	return &MultiCache{
		first:  first,
		second: second,
	}
}

// Get looks for an item in all the caches one by one and returns the Cache found item.
func (mc *MultiCache) Get(key shard.Key) (Accessor, error) {
	ac, err := mc.first.Get(key)
	if err == nil {
		return ac, nil
	}
	return mc.second.Get(key)
}

// GetOrLoad attempts to get an item from all caches, and if not found, invokes
// the provided loader function to load it into one of the caches.
func (mc *MultiCache) GetOrLoad(
	ctx context.Context,
	key shard.Key,
	loader func(context.Context, shard.Key) (AccessorProvider, error),
) (Accessor, error) {
	ac, err := mc.first.GetOrLoad(ctx, key, loader)
	if err == nil {
		return ac, nil
	}
	return mc.second.GetOrLoad(ctx, key, loader)
}

// Remove removes an item from all underlying caches
func (mc *MultiCache) Remove(key shard.Key) error {
	err1 := mc.first.Remove(key)
	err2 := mc.second.Remove(key)
	return errors.Join(err1, err2)
}

func (mc *MultiCache) EnableMetrics() error {
	if err := mc.first.EnableMetrics(); err != nil {
		return err
	}
	return mc.second.EnableMetrics()
}
