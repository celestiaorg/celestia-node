package pruner

import (
	"context"
	"time"

	"github.com/celestiaorg/celestia-node/header"
)

// maxHeadersPerLoop is the maximum number of headers to fetch
// for a prune loop (prevents fetching too many headers at a
// time for nodes that have a large number of pruneable headers).
var maxHeadersPerLoop = uint64(1024)

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

	log.Debugw("finder: fetching header range", "last pruned", lastPruned.Height(),
		"target height", estimatedCutoffHeight)

	if estimatedCutoffHeight-lastPruned.Height() > maxHeadersPerLoop {
		estimatedCutoffHeight = lastPruned.Height() + maxHeadersPerLoop
	}

	headers, err := s.getter.GetRangeByHeight(ctx, lastPruned, estimatedCutoffHeight)
	if err != nil {
		return nil, err
	}
	// ensures genesis block gets pruned
	if lastPruned.Height() == 1 {
		headers = append([]*header.ExtendedHeader{lastPruned}, headers...)
	}

	// if our estimated range didn't cover enough headers, we need to fetch more
	// TODO: This is really inefficient in the case that lastPruned is the default value, or if the
	// node has been offline for a long time. Instead of increasing the boundary by one in the for
	// loop we could increase by a range every iteration
	headerCount := len(headers)
	for {
		if headerCount > int(maxHeadersPerLoop) {
			headers = headers[:maxHeadersPerLoop]
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
