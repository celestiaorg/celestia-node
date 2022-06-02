package share

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

var shareStorePrefix = datastore.NewKey("shares")

func rootKey(root *Root) datastore.Key {
	return datastore.NewKey(root.String())
}

// localAvailability wraps a given Availability (whether it's light or full)
// and stores the results of a successful sampling routine over a given Root's hash
// to disk.
type localAvailability struct {
	avail Availability

	ds datastore.Datastore
}

// NewLocalAvailability wraps the given Availability with an additional datastore
// for sampling result caching.
func NewLocalAvailability(avail Availability, ds datastore.Datastore) (Availability, error) {
	ds = namespace.Wrap(ds, shareStorePrefix)
	return &localAvailability{
		avail: avail,
		ds:    ds,
	}, nil
}

// SharesAvailable will store, upon success, the hash of the given Root to disk.
func (la localAvailability) SharesAvailable(ctx context.Context, root *Root) error {
	// do not sample over Root that has already been sampled
	exists, err := la.ds.Has(rootKey(root))
	if err != nil {
		return err
	}
	if exists {
		// root has already been sampled successfully
		return nil
	}
	err = la.avail.SharesAvailable(ctx, root)
	if err != nil {
		return err
	}
	err = la.ds.Put(rootKey(root), []byte{})
	if err != nil {
		log.Errorw("storing result of successful SharesAvailable request to disk", "err", err)
	}
	return err
}
