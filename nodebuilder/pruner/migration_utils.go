package pruner

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"

	fullavail "github.com/celestiaorg/celestia-node/share/availability/full"
)

// TODO @renaynay: remove this file after a few releases -- this utility serves as a temporary solution
// to detect if the node has been run with pruning enabled before on previous version(s), and disallow
// running as an archival node.

var (
	storePrefix     = datastore.NewKey("full_avail")
	previousModeKey = datastore.NewKey("previous_run")
)

// detectFirstRun is a temporary function that serves to assist migration to the refactored pruner
// implementation (v0.21.0). It checks if the node has been run with pruning enabled before by checking
// if the pruner service ran before, and disallows running as an archival node in the case it has.
func detectFirstRun(ctx context.Context, cfg *Config, ds datastore.Datastore, lastPrunedHeight uint64) error {
	ds = namespace.Wrap(ds, storePrefix)

	exists, err := ds.Has(ctx, previousModeKey)
	if err != nil {
		return fmt.Errorf("share/availability/full: failed to check previous pruned run in "+
			"datastore: %w", err)
	}
	if exists {
		// node has already been run on current version, no migration is necessary
		return nil
	}

	isArchival := !cfg.EnableService

	// if the node has been pruned before on a previous version, it cannot revert
	// to archival mode
	if isArchival && lastPrunedHeight > 1 {
		return fullavail.ErrDisallowRevertToArchival
	}

	return recordFirstRun(ctx, ds, isArchival)
}

// recordFirstRun exists to assist migration to new pruner implementation (v0.21.0) by recording
// the first run of the pruner service in the full availability's datastore. It assumes the datastore
// is already namespace-wrapped.
func recordFirstRun(ctx context.Context, ds datastore.Datastore, isArchival bool) error {
	mode := []byte("pruned")
	if isArchival {
		mode = []byte("archival")
	}

	return ds.Put(ctx, previousModeKey, mode)
}
