package pruner

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-datastore"

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
	lastPruned := s.lastPruned()
	pruneCutoff := time.Now().Add(time.Duration(-s.window))
	estimatedCutoffHeight := lastPruned.Height() + numBlocksInWindow

	headers, err := s.getter.GetRangeByHeight(ctx, lastPruned, estimatedCutoffHeight)
	if err != nil {
		return nil, err
	}

	// if our estimated range didn't cover enough headers, we need to fetch more
	for {
		lastHeader := headers[len(headers)-1]
		if lastHeader.Time().After(pruneCutoff) {
			break
		}

		nextHeader, err := s.getter.GetByHeight(ctx, lastHeader.Height()+1)
		if err != nil {
			return nil, err
		}
		headers = append(headers, nextHeader)
	}

	for i, h := range headers {
		if h.Time().After(pruneCutoff) {
			if i == 0 {
				// we can't prune anything
				return nil, nil
			}

			// we can ignore the rest of the headers since they are all newer than the cutoff
			return headers[:i-1], nil
		}
	}
	return headers, nil
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
