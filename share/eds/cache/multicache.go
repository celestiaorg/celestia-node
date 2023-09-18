package cache

import (
	"errors"

	"github.com/filecoin-project/dagstore/shard"
)

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

// Remove removes an item from all underlying caches
func (mc *MultiCache) Remove(key shard.Key) error {
	err1 := mc.first.Remove(key)
	err2 := mc.second.Remove(key)
	return errors.Join(err1, err2)
}

func (mc *MultiCache) First() Cache {
	return mc.first
}

func (mc *MultiCache) Second() Cache {
	return mc.second
}

func (mc *MultiCache) EnableMetrics() error {
	if err := mc.first.EnableMetrics(); err != nil {
		return err
	}
	return mc.second.EnableMetrics()
}
