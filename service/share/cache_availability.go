package share

import (
	"bytes"
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/service/share/discovery"
)

var (
	// DefaultWriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	// TODO(@Wondertan, @renaynay): Those values must be configurable and proper defaults should be set for specific node
	//  type. (#709)
	DefaultWriteBatchSize   = 2048
	cacheAvailabilityPrefix = datastore.NewKey("sampling_result")

	minRoot = da.MinDataAvailabilityHeader()
)

// CacheAvailability wraps a given Availability (whether it's light or full)
// and stores the results of a successful sampling routine over a given Root's hash
// to disk.
type CacheAvailability struct {
	avail Availability

	cancel context.CancelFunc

	ds         *autobatch.Datastore
	discoverer *discovery.Discoverer
}

// NewCacheAvailability wraps the given Availability with an additional datastore
// for sampling result caching.
func NewCacheAvailability(avail Availability, ds datastore.Batching, d *discovery.Discoverer) *CacheAvailability {
	ds = namespace.Wrap(ds, cacheAvailabilityPrefix)
	autoDS := autobatch.NewAutoBatching(ds, DefaultWriteBatchSize)

	return &CacheAvailability{
		avail:      avail,
		ds:         autoDS,
		discoverer: d,
	}
}

// SharesAvailable will store, upon success, the hash of the given Root to disk.
func (ca *CacheAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	// short-circuit if the given root is minimum DAH of an empty data square
	if isMinRoot(root) {
		return nil
	}
	// do not sample over Root that has already been sampled
	key := rootKey(root)
	exists, err := ca.ds.Has(key)
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
	err = ca.ds.Put(key, []byte{})
	if err != nil {
		log.Errorw("storing root of successful SharesAvailable request to disk", "err", err)
	}
	return err
}

// Start starts looking for a new peers in network.
func (ca *CacheAvailability) Start(context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	ca.cancel = cancel
	ca.discoverer.Start(ctx)
	return nil
}

// Close flushes all queued writes to disk.
func (ca *CacheAvailability) Close(context.Context) error {
	ca.cancel()
	return ca.ds.Flush()
}

func rootKey(root *Root) datastore.Key {
	return datastore.NewKey(root.String())
}

// isMinRoot returns whether the given root is a minimum (empty)
// DataAvailabilityHeader (DAH).
func isMinRoot(root *Root) bool {
	return bytes.Equal(minRoot.Hash(), root.Hash())
}
