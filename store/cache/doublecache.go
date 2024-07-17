package cache

import (
	"errors"
	"fmt"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
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
func (mc *DoubleCache) Get(height uint64) (eds.AccessorStreamer, error) {
	accessor, err := mc.first.Get(height)
	if err == nil {
		return accessor, nil
	}
	return mc.second.Get(height)
}

// Remove removes an item from all underlying caches
func (mc *DoubleCache) Remove(height uint64) error {
	err1 := mc.first.Remove(height)
	err2 := mc.second.Remove(height)
	return errors.Join(err1, err2)
}

func (mc *DoubleCache) First() Cache {
	return mc.first
}

func (mc *DoubleCache) Second() Cache {
	return mc.second
}

func (mc *DoubleCache) EnableMetrics() (unreg func() error, err error) {
	unreg1, err := mc.first.EnableMetrics()
	if err != nil {
		return nil, fmt.Errorf("while enabling metrics for first cache: %w", err)
	}
	unreg2, err := mc.second.EnableMetrics()
	if err != nil {
		return unreg1, fmt.Errorf("while enabling metrics for second cache: %w", err)
	}

	unregFn := func() error {
		return errors.Join(unreg1(), unreg2())
	}
	return unregFn, nil
}
