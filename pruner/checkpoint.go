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
	storePrefix           = datastore.NewKey("pruner")
	checkpointKey         = datastore.NewKey("checkpoint")
	errCheckpointNotFound = errors.New("checkpoint not found")
)

// checkpoint contains information related to the state of the
// pruner service that is periodically persisted to disk.
type checkpoint struct {
	LastPrunedHeight uint64              `json:"last_pruned_height"`
	FailedHeaders    map[uint64]struct{} `json:"failed"`
}

func newCheckpoint() *checkpoint {
	return &checkpoint{
		LastPrunedHeight: 1,
		FailedHeaders:    map[uint64]struct{}{},
	}
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
			s.checkpoint = newCheckpoint()
			return storeCheckpoint(ctx, s.ds, s.checkpoint)
		}
		return err
	}

	s.checkpoint = cp
	return nil
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
