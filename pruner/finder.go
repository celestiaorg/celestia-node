package pruner

import (
	"context"
	"github.com/ipfs/go-datastore"
	"sync/atomic"

	"github.com/celestiaorg/celestia-node/header"
)

var (
	lastPrunedHeaderKey = datastore.NewKey("last_pruned_header")

	numBlocksInWindow uint64
)

type checkpoint struct {
	ds datastore.Datastore

	lastPrunedHeader atomic.Pointer[header.ExtendedHeader]
}

func newCheckpoint(ds datastore.Datastore) *checkpoint {
	return &checkpoint{ds: ds}
}

// findPruneableHeaders returns all headers that are eligible for pruning
// (outside the sampling window).
func (s *Service) findPruneableHeaders(ctx context.Context) ([]*header.ExtendedHeader, error) {
}

// updateCheckpoint updates the checkpoint with the last pruned header height
// and persists it to disk.
func (s *Service) updateCheckpoint(ctx context.Context, lastPruned *header.ExtendedHeader) error {
	s.checkpoint.lastPrunedHeader.Store(lastPruned)

	bin, err := lastPruned.MarshalJSON()
	if err != nil {
		return err
	}

	return s.checkpoint.ds.Put(ctx, lastPrunedHeaderKey, bin)
}

func (s *Service) lastPruned() *header.ExtendedHeader {
	return s.checkpoint.lastPrunedHeader.Load()
}
