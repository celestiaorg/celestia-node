package pruner

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

// maxHeadersPerLoop is the maximum number of headers to fetch
// for a prune loop (prevents fetching too many headers at a
// time for nodes that have a large number of pruneable headers).
var maxHeadersPerLoop = 512

// findPruneableHeaders returns all headers that are eligible for pruning
// (outside the sampling window).
func (s *Service) findPruneableHeaders(
	ctx context.Context,
	lastPruned *header.ExtendedHeader,
) ([]*header.ExtendedHeader, error) {
	// TODO(@Wondertan): This should be SyncerHead to ensure the most accurate Head
	//  but its ok for now and will be fixed in a future release.
	head, err := s.hstore.Head(ctx)
	if err != nil {
		return nil, err
	}

	pruneCutoff := head.Time().UTC().Add(-s.window)

	if !lastPruned.Time().UTC().Before(pruneCutoff) {
		// this can happen when the network is young and all blocks
		// are still within the AvailabilityWindow
		return nil, nil
	}

	estimatedCutoffHeight, err := s.calculateEstimatedCutoff(ctx, lastPruned, pruneCutoff)
	if err != nil {
		return nil, err
	}

	if lastPruned.Height() >= estimatedCutoffHeight {
		// nothing left to prune
		return nil, nil
	}

	log.Debugw("finder: fetching header range", "last pruned", lastPruned.Height(),
		"target height", estimatedCutoffHeight)

	// GetRangeByHeight requests (from:to), where `to` is non-inclusive, we need
	// to request one more header than the estimated cutoff
	headers, err := s.hstore.GetRangeByHeight(ctx, lastPruned, estimatedCutoffHeight+1)
	if err != nil {
		log.Errorw("failed to get range from header store", "from", lastPruned.Height(),
			"to", estimatedCutoffHeight, "error", err)
		return nil, err
	}
	// ensures genesis block gets pruned
	if lastPruned.Height() == 1 {
		headers = append([]*header.ExtendedHeader{lastPruned}, headers...)
	}

	// if our estimated range didn't cover enough headers, we need to fetch more
	headerCount := len(headers)
	if headerCount < maxHeadersPerLoop {
		lastHeader := headers[len(headers)-1]
		if !lastHeader.Time().After(pruneCutoff) {
			remaining := maxHeadersPerLoop - headerCount
			maxToHeight := lastHeader.Height() + uint64(remaining) + 1
			headHeightPlusOne := head.Height() + 1
			if maxToHeight > headHeightPlusOne {
				maxToHeight = headHeightPlusOne
			}
			if maxToHeight > lastHeader.Height()+1 {
				moreHeaders, err := s.hstore.GetRangeByHeight(ctx, lastHeader, maxToHeight)
				if err != nil {
					log.Errorw("failed to get additional range from header store", "from", lastHeader.Height(),
						"to", maxToHeight-1, "error", err)
					return nil, err
				}
				headers = append(headers, moreHeaders...)
			}
		}
	}
	if len(headers) > maxHeadersPerLoop {
		headers = headers[:maxHeadersPerLoop]
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

func (s *Service) calculateEstimatedCutoff(
	ctx context.Context,
	lastPruned *header.ExtendedHeader,
	pruneCutoff time.Time,
) (uint64, error) {
	estimatedRange := uint64(pruneCutoff.UTC().Sub(lastPruned.Time().UTC()) / s.blockTime)
	estimatedCutoffHeight := lastPruned.Height() + estimatedRange

	head, err := s.hstore.Head(ctx)
	if err != nil {
		log.Errorw("failed to get Head from header store", "error", err)
		return 0, err
	}

	if head.Height() < estimatedCutoffHeight {
		estimatedCutoffHeight = head.Height()
	}

	if estimatedCutoffHeight-lastPruned.Height() > uint64(maxHeadersPerLoop) {
		estimatedCutoffHeight = lastPruned.Height() + uint64(maxHeadersPerLoop)
	}

	return estimatedCutoffHeight, nil
}
