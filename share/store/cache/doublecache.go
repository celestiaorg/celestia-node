package cache

import (
	"errors"

	"github.com/celestiaorg/celestia-node/share/store/file"
)

// DoubleCache represents a Cache that looks into multiple caches one by one.
type DoubleCache struct {
	first, second Cache
}

// NewDoubleCache creates a new DoubleCache with the provided caches.
func NewDoubleCache(first, second Cache) *DoubleCache {
	return &DoubleCache{
		first:  first,
		second: second,
	}
}

// Get looks for an item in all the caches one by one and returns the Cache found item.
func (mc *DoubleCache) Get(key key) (file.EdsFile, error) {
	accessor, err := mc.first.Get(key)
	if err == nil {
		return accessor, nil
	}
	return mc.second.Get(key)
}

// Remove removes an item from all underlying caches
func (mc *DoubleCache) Remove(key key) error {
	err1 := mc.first.Remove(key)
	err2 := mc.second.Remove(key)
	return errors.Join(err1, err2)
}

func (mc *DoubleCache) First() Cache {
	return mc.first
}

func (mc *DoubleCache) Second() Cache {
	return mc.second
}

func (mc *DoubleCache) EnableMetrics() error {
	if err := mc.first.EnableMetrics(); err != nil {
		return err
	}
	return mc.second.EnableMetrics()
}
