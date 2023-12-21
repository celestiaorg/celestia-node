package pruner

import (
	"context"
	"fmt"
	"github.com/ipfs/go-datastore"
	"sync/atomic"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

var lastPrunedHeaderKey = datastore.NewKey("last_pruned_header")

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
	// 1. 	take last pruned header timestamp
	// 2. 	estimate availability window last header based on time and block time
	// 		estimation (header height based on last header)
	// 3. 	load header from s.getter, check timestamp
	// 4. 	begin algorithm to find header
	// 5. 	once found, return headers in ascending order [lowest -> highest]

	// TODO @renaynay: update this algo with the above ^
	lastPruned := s.lastPruned()

	elapsedTimeSincePruned := time.Since(lastPruned.Time())
	elapsedBlocksSincePruned := elapsedTimeSincePruned / s.blockTime
	estimatedNextPrunedHeight := lastPruned.Height() + uint64(elapsedBlocksSincePruned)
	fmt.Println("41:         ", estimatedNextPrunedHeight)

	next, err := s.getter.GetRangeByHeight(ctx, lastPruned, estimatedNextPrunedHeight) // TODO @renaynay: make sure this is the non-blocking call?
	if err != nil {
		return nil, err
	}
	return next, nil
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
