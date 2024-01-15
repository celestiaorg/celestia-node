package pruner

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/header"
)

var (
	lastPrunedHeaderKey = datastore.NewKey("last_pruned_header")
)

type checkpoint struct {
	ds datastore.Datastore

	lastPrunedHeader atomic.Pointer[header.ExtendedHeader]

	// TODO @renaynay: keep track of failed roots to retry  in separate job
}

func newCheckpoint(ds datastore.Datastore) *checkpoint {
	return &checkpoint{ds: ds}
}

// findPruneableHeaders returns all headers that are eligible for pruning
// (outside the sampling window).
func (s *Service) findPruneableHeaders(ctx context.Context) ([]*header.ExtendedHeader, error) {
	lastPruned := s.lastPruned()

	pruneCutoff := time.Now().Add(time.Duration(-s.window))
	estimatedCutoffHeight := lastPruned.Height() + s.numBlocksInWindow

	head, err := s.getter.Head(ctx)
	if err != nil {
		return nil, err
	}
	if head.Height() < estimatedCutoffHeight {
		estimatedCutoffHeight = head.Height()
	}

	log.Debugw("finder: fetching header range", "lastPruned", lastPruned.Height(),
		"estimatedCutoffHeight", estimatedCutoffHeight)

	headers, err := s.getter.GetRangeByHeight(ctx, lastPruned, estimatedCutoffHeight)
	if err != nil {
		return nil, err
	}

	// if our estimated range didn't cover enough headers, we need to fetch more
	// TODO: This is really inefficient in the case that lastPruned is the default value, or if the
	// node has been offline for a long time. Instead of increasing the boundary by one in the for
	// loop we could increase by a range every iteration
	headerCount := len(headers)
	for {
		if headerCount > int(s.maxPruneablePerGC) {
			headers = headers[:s.maxPruneablePerGC]
			break
		}
		lastHeader := headers[len(headers)-1]
		if lastHeader.Time().After(pruneCutoff) {
			break
		}

		nextHeader, err := s.getter.GetByHeight(ctx, lastHeader.Height()+1)
		if err != nil {
			return nil, err
		}
		headers = append(headers, nextHeader)
		headerCount++
	}

	for i, h := range headers {
		if h.Time().After(pruneCutoff) {
			if i == 0 {
				// we can't prune anything
				return nil, nil
			}

			// we can ignore the rest of the headers since they are all newer than the cutoff
			return headers[:i], nil
		}
	}
	return headers, nil
}

// initializeCheckpoint initializes the checkpoint, storing the earliest header in the chain.
func (s *Service) initializeCheckpoint(ctx context.Context) error {
	firstHeader, err := s.getter.GetByHeight(ctx, 1)
	if err != nil {
		return fmt.Errorf("failed to initialize checkpoint: %w", err)
	}

	return s.updateCheckpoint(ctx, firstHeader)
}

// loadCheckpoint loads the last checkpoint from disk, initializing it if it does not already exist.
func (s *Service) loadCheckpoint(ctx context.Context) error {
	bin, err := s.checkpoint.ds.Get(ctx, lastPrunedHeaderKey)
	if err != nil {
		if err == datastore.ErrNotFound {
			return s.initializeCheckpoint(ctx)
		}
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}

	var lastPruned header.ExtendedHeader
	if err := lastPruned.UnmarshalJSON(bin); err != nil {
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}

	s.checkpoint.lastPrunedHeader.Store(&lastPruned)
	return nil
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
