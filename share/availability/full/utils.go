package full

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

var (
	storePrefix     = datastore.NewKey("full_avail")
	previousModeKey = datastore.NewKey("previous_mode")
	pruned          = []byte("pruned")
	archival        = []byte("archival")
)

// ConvertFromArchivalToPruned ensures that a node has not been run with pruning enabled before
// cannot revert to archival mode. It returns true only if the node is converting to
// pruned mode for the first time.
func ConvertFromArchivalToPruned(ctx context.Context, ds datastore.Datastore, isArchival bool) (bool, error) {
	ds = namespace.Wrap(ds, storePrefix)

	prevMode, err := ds.Get(ctx, previousModeKey)
	if err != nil {
		return false, err
	}

	if bytes.Equal(prevMode, pruned) && isArchival {
		return false, ErrDisallowRevertToArchival
	}

	if bytes.Equal(prevMode, archival) && !isArchival {
		// allow conversion from archival to pruned
		err = ds.Put(ctx, previousModeKey, pruned)
		if err != nil {
			return false, fmt.Errorf("share/availability/full: failed to updated pruning mode in "+
				"datastore: %w", err)
		}
		return true, nil
	}

	// no changes in pruning mode
	return false, nil
}
