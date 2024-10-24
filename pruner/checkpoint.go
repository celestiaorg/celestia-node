package pruner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/header"
)

var (
	// ErrDisallowRevertToArchival is returned when a node has been run with pruner enabled before and
	// launching it with archival mode.
	ErrDisallowRevertToArchival = errors.New(
		"node has been run with pruner enabled before, it is not safe to convert to an archival" +
			"Run with --experimental-pruning enabled or consider re-initializing the store")

	storePrefix           = datastore.NewKey("pruner")
	checkpointKey         = datastore.NewKey("checkpoint")
	errCheckpointNotFound = errors.New("checkpoint not found")
)

// checkpoint contains information related to the state of the
// pruner service that is periodically persisted to disk.
type checkpoint struct {
	PrunerType       string              `json:"pruner_type"`
	LastPrunedHeight uint64              `json:"last_pruned_height"`
	FailedHeaders    map[uint64]struct{} `json:"failed"`
}

// DetectPreviousRun checks if the pruner has run before by checking for the existence of a
// checkpoint.
func DetectPreviousRun(_ context.Context, _ datastore.Datastore) error {
	/*
		_, err := getCheckpoint(ctx, namespace.Wrap(ds, storePrefix))
		if errors.Is(err, errCheckpointNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to load checkpoint: %w", err)
		}
		return ErrDisallowRevertToArchival
	*/
	// TODO @renaynay: there needs to be some sort of way for an archival node to go --> pruned node now bc
	//  it'll contain the checkpoint so the pruner will think the blocks have already been fully pruneed. Maybe we add
	//  some additional info to the checkpoint to indicate that it's an archival node? or the previous mode of the run.
	return nil
}

// storeCheckpoint persists the checkpoint to disk.
func storeCheckpoint(ctx context.Context, ds datastore.Datastore, c *checkpoint) error {
	bin, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return ds.Put(ctx, checkpointKey, bin)
}

// getCheckpoint loads the last checkpoint from disk.
func getCheckpoint(ctx context.Context, ds datastore.Datastore) (*checkpoint, error) {
	bin, err := ds.Get(ctx, checkpointKey)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, errCheckpointNotFound
		}
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	var cp *checkpoint
	err = json.Unmarshal(bin, &cp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return cp, nil
}

// loadCheckpoint loads the last checkpoint from disk, initializing it if it does not already exist.
func (s *Service) loadCheckpoint(ctx context.Context) error {
	cp, err := getCheckpoint(ctx, s.ds)
	if err != nil {
		if errors.Is(err, errCheckpointNotFound) {
			s.checkpoint = &checkpoint{
				PrunerType:       s.pruner.Kind(),
				LastPrunedHeight: 1,
				FailedHeaders:    map[uint64]struct{}{},
			}
			return storeCheckpoint(ctx, s.ds, s.checkpoint)
		}
		return err
	}

	// ensure that the checkpoint is of the same pruner type as the current
	// pruner
	if cp.PrunerType == s.pruner.Kind() {
		s.checkpoint = cp
		return nil
	}
	// only a transition from archival --> full is allowed
	if cp.PrunerType == "archival" && s.pruner.Kind() == "full" {
		// reset the checkpoint
		cp = &checkpoint{
			PrunerType:       s.pruner.Kind(),
			LastPrunedHeight: 1,
			FailedHeaders:    make(map[uint64]struct{}),
		}

		s.checkpoint = cp
		return nil
	}

	return fmt.Errorf("pruner: mismatched pruner type provided - only a "+
		"transition from archival -> pruned node is allowed. Previous run: %s, "+
		"current run: %s", cp.PrunerType, s.pruner.Kind())
}

// updateCheckpoint updates the checkpoint with the last pruned header height
// and persists it to disk.
func (s *Service) updateCheckpoint(
	ctx context.Context,
	lastPrunedHeight uint64,
	failedHeights map[uint64]struct{},
) error {
	for height := range failedHeights {
		s.checkpoint.FailedHeaders[height] = struct{}{}
	}

	s.checkpoint.LastPrunedHeight = lastPrunedHeight
	return storeCheckpoint(ctx, s.ds, s.checkpoint)
}

func (s *Service) lastPruned(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.getter.GetByHeight(ctx, s.checkpoint.LastPrunedHeight)
}
