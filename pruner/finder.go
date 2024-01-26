package pruner

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

// findPruneableHeaders returns all headers that are eligible for pruning
// (outside the sampling window).
// TODO @renaynay @distractedm1nd: This will not prune the genesis block
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
