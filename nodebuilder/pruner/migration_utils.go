package pruner

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"

	fullavail "github.com/celestiaorg/celestia-node/share/availability/full"
)

var (
	storePrefix     = datastore.NewKey("full_avail")
	previousModeKey = datastore.NewKey("previous_run")
	pruned          = []byte("pruned")
	archival        = []byte("archival")
)

// convertFromArchivalToPruned ensures that a node has not been run with pruning enabled before
// cannot revert to archival mode. It returns true only if the node is converting to
// pruned mode for the first time.
func convertFromArchivalToPruned(ctx context.Context, cfg *Config, ds datastore.Datastore) (bool, error) {
	prevMode, err := ds.Get(ctx, previousModeKey)
	if err != nil {
		return false, err
	}

	if bytes.Equal(prevMode, pruned) && !cfg.EnableService {
		return false, fullavail.ErrDisallowRevertToArchival
	}

	if bytes.Equal(prevMode, archival) && cfg.EnableService {
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

// detectFirstRun is a temporary function that serves to assist migration to the refactored pruner
// implementation (v0.21.0). It checks if the node has been run with pruning enabled before by checking
// if the pruner service ran before, and disallows running as an archival node in the case it has.
//
// TODO @renaynay: remove this function after a few releases.
func detectFirstRun(ctx context.Context, cfg *Config, ds datastore.Datastore, lastPrunedHeight uint64) error {
	exists, err := ds.Has(ctx, previousModeKey)
	if err != nil {
		return fmt.Errorf("share/availability/full: failed to check previous pruned run in "+
			"datastore: %w", err)
	}
	if exists {
		return nil
	}

	if !cfg.EnableService {
		if lastPrunedHeight > 1 {
			return fullavail.ErrDisallowRevertToArchival
		}

		return ds.Put(ctx, previousModeKey, archival)
	}

	return ds.Put(ctx, previousModeKey, pruned)
}
