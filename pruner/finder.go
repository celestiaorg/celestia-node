package pruner

import (
	"context"
	"encoding/binary"
	"github.com/ipfs/go-datastore"
	"sync/atomic"

	"github.com/celestiaorg/celestia-node/header"
)

var lastPrunedHeaderKey = datastore.NewKey("last_pruned_header")

type checkpoint struct {
	ds datastore.Datastore

	lastPrunedHeaderHeight atomic.Uint64
}

// findPruneableHeaders returns all headers that are eligible for pruning
// (outside the sampling window).
func (s *Service) findPruneableHeaders() ([]*header.ExtendedHeader, error) {
	// 1. 	take last pruned header timestamp
	// 2. 	estimate availability window last header based on time and block time
	// 		estimation (header height based on last header)
	// 3. 	load header from s.getter, check timestamp
	// 4. 	begin algorithm to find header
	// 5. 	once found, return headers in ascending order [lowest -> highest]
	return nil, nil
}

func (s *Service) updateCheckpoint(ctx context.Context, lastHeight uint64) error {
	// update checkpoint
	s.checkpoint.lastPrunedHeaderHeight.Store(lastHeight)
	// dump to disk
	bin := make([]byte, 8)
	binary.BigEndian.PutUint64(bin[:], lastHeight)
	return s.checkpoint.ds.Put(ctx, lastPrunedHeaderKey, bin)
}

func (s *Service) lastPrunedHeight() uint64 {
	return s.checkpoint.lastPrunedHeaderHeight.Load()
}
