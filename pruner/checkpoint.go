package pruner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/header"
)

var (
	storePrefix   = datastore.NewKey("pruner")
	checkpointKey = datastore.NewKey("checkpoint")
)

// checkpoint contains information related to the state of the
// pruner service that is periodically persisted to disk.
type checkpoint struct {
	LastPrunedHeight uint64              `json:"last_pruned_height"`
	FailedHeaders    map[uint64]struct{} `json:"failed"`
}

// initializeCheckpoint initializes the checkpoint, storing the earliest header in the chain.
func (s *Service) initializeCheckpoint(ctx context.Context) error {
	return s.updateCheckpoint(ctx, uint64(1), nil)
}

// loadCheckpoint loads the last checkpoint from disk, initializing it if it does not already exist.
func (s *Service) loadCheckpoint(ctx context.Context) error {
	bin, err := s.ds.Get(ctx, checkpointKey)
	if err != nil {
		if err == datastore.ErrNotFound {
			return s.initializeCheckpoint(ctx)
		}
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}

	var cp *checkpoint
	err = json.Unmarshal(bin, &cp)
	if err != nil {
		return fmt.Errorf("failed to unmarshal checkpoint: %w", err)
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

	bin, err := json.Marshal(s.checkpoint)
	if err != nil {
		return err
	}

	return s.ds.Put(ctx, checkpointKey, bin)
}

func (s *Service) lastPruned(ctx context.Context) (*header.ExtendedHeader, error) {
	return s.getter.GetByHeight(ctx, s.checkpoint.LastPrunedHeight)
}
