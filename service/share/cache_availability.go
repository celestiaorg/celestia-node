package share

import (
	"context"

	lru "github.com/hashicorp/golang-lru"

	"github.com/celestiaorg/celestia-node/header/store"
)

// cacheAvailability wraps a given Availability (whether it's light or full)
// and caches the results of a successful sampling routine over a given Root's hash.
type cacheAvailability struct {
	avail Availability

	// adaptive replacement cache of shares
	cache *lru.ARCCache
}

// NewCacheAvailability wraps the given Availability with a cache.
func NewCacheAvailability(avail Availability) (Availability, error) {
	cache, err := lru.NewARC(store.DefaultStoreCacheSize)
	if err != nil {
		return nil, err
	}
	return &cacheAvailability{
		avail: avail,
		cache: cache,
	}, nil
}

// SharesAvailable will cache, upon success, the hash of the given Root.
func (ca cacheAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	// if root has already been sampled successfully, do not
	// sample again
	_, ok := ca.cache.Get(root.String())
	if ok {
		return nil
	}
	if err := ca.avail.SharesAvailable(ctx, root); err != nil {
		return err
	}
	// TODO @renaynay: what gets stored here? an empty byte arr /struct or something to indicate that it has been sampled?
	ca.cache.Add(root.String(), struct{}{})
	return nil
}

// Stop purges the cache.
func (ca *cacheAvailability) Stop(context.Context) error {
	ca.cache.Purge()
	return nil
}
