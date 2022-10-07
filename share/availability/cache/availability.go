package cache

import (
	"bytes"
	"context"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/autobatch"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logging.Logger("share/availability/cache")

var (
	// DefaultWriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	// TODO(@Wondertan, @renaynay): Those values must be configurable and proper defaults should be set for specific node
	//  type. (#709)
	DefaultWriteBatchSize   = 2048
	cacheAvailabilityPrefix = datastore.NewKey("sampling_result")

	minRoot = da.MinDataAvailabilityHeader()
)

// ShareAvailability wraps a given share.Availability (whether it's light or full)
// and stores the results of a successful sampling routine over a given Root's hash
// to disk.
type ShareAvailability struct {
	avail share.Availability

	// TODO(@Wondertan): Once we come to parallelized DASer, this lock becomes a contention point
	//  Related to #483
	dsLk sync.RWMutex
	ds   *autobatch.Datastore
}

// NewShareAvailability wraps the given share.Availability with an additional datastore
// for sampling result caching.
func NewShareAvailability(avail share.Availability, ds datastore.Batching) *ShareAvailability {
	ds = namespace.Wrap(ds, cacheAvailabilityPrefix)
	autoDS := autobatch.NewAutoBatching(ds, DefaultWriteBatchSize)

	return &ShareAvailability{
		avail: avail,
		ds:    autoDS,
	}
}

// SharesAvailable will store, upon success, the hash of the given Root to disk.
func (ca *ShareAvailability) SharesAvailable(ctx context.Context, root *share.Root) error {
	// short-circuit if the given root is minimum DAH of an empty data square
	if isMinRoot(root) {
		return nil
	}
	// do not sample over Root that has already been sampled
	key := rootKey(root)

	ca.dsLk.RLock()
	exists, err := ca.ds.Has(ctx, key)
	ca.dsLk.RUnlock()
	if err != nil || exists {
		return err
	}

	err = ca.avail.SharesAvailable(ctx, root)
	if err != nil {
		return err
	}

	ca.dsLk.Lock()
	err = ca.ds.Put(ctx, key, []byte{})
	ca.dsLk.Unlock()
	if err != nil {
		log.Errorw("storing root of successful SharesAvailable request to disk", "err", err)
	}
	return err
}

func (ca *ShareAvailability) ProbabilityOfAvailability() float64 {
	return ca.avail.ProbabilityOfAvailability()
}

// Close flushes all queued writes to disk.
func (ca *ShareAvailability) Close(ctx context.Context) error {
	return ca.ds.Flush(ctx)
}

func rootKey(root *share.Root) datastore.Key {
	return datastore.NewKey(root.String())
}

// isMinRoot returns whether the given root is a minimum (empty)
// DataAvailabilityHeader (DAH).
func isMinRoot(root *share.Root) bool {
	return bytes.Equal(minRoot.Hash(), root.Hash())
}
