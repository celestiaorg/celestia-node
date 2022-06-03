package share

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"

	"github.com/celestiaorg/celestia-node/header/store"
)

var cacheAvailabilityPrefix = datastore.NewKey("sampling_result")

func rootKey(root *Root) datastore.Key {
	return datastore.NewKey(root.String())
}

// cacheAvailability wraps a given Availability (whether it's light or full)
// and stores the results of a successful sampling routine over a given Root's hash
// to disk.
type cacheAvailability struct {
	avail Availability

	ds *autobatch.Datastore
}

// NewCacheAvailability wraps the given Availability with an additional datastore
// for sampling result caching.
func NewCacheAvailability(avail Availability, ds datastore.Batching) Availability {
	ds = namespace.Wrap(ds, cacheAvailabilityPrefix)
	autoDS := autobatch.NewAutoBatching(ds, store.DefaultWriteBatchSize)
	return &cacheAvailability{
		avail: avail,
		ds:    autoDS,
	}
}

// SharesAvailable will store, upon success, the hash of the given Root to disk.
func (ca *cacheAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	// do not sample over Root that has already been sampled
	exists, err := ca.ds.Has(rootKey(root))
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = ca.avail.SharesAvailable(ctx, root)
	if err != nil {
		return err
	}
	err = ca.ds.Put(rootKey(root), []byte{})
	if err != nil {
		log.Errorw("storing result of successful SharesAvailable request to disk", "err", err)
	}
	return err
}
