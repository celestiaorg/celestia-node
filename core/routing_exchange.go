package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

// RoutingExchange combines core and P2P exchanges to optimize header syncing for Bridge nodes.
// It routes requests based on whether headers fall within the availability window:
//   - Inside window: uses core exchange (fetches full blocks from consensus)
//   - Outside window: uses P2P exchange (fetches headers only from network)
type RoutingExchange struct {
	core libhead.Exchange[*header.ExtendedHeader]
	p2p  libhead.Exchange[*header.ExtendedHeader]

	// window is the storage window duration used to calculate the cutoff height.
	window time.Duration
	// blockTime is the expected block production time.
	blockTime time.Duration
}

// NewRoutingExchange creates a new RoutingExchange that wraps core and P2P exchanges.
func NewRoutingExchange(
	coreEx libhead.Exchange[*header.ExtendedHeader],
	p2pEx libhead.Exchange[*header.ExtendedHeader],
	window time.Duration,
	blockTime time.Duration,
) *RoutingExchange {
	h := &RoutingExchange{
		core:      coreEx,
		p2p:       p2pEx,
		window:    window,
		blockTime: blockTime,
	}
	return h
}

// Head returns the latest header from the core exchange.
// We always use core for Head() to get the authoritative latest block.
// Also updates the cached cutoff height for routing decisions.
func (h *RoutingExchange) Head(
	ctx context.Context,
	opts ...libhead.HeadOption[*header.ExtendedHeader],
) (*header.ExtendedHeader, error) {
	head, err := h.core.Head(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("requesting head from consensus nod: %w", err)
	}

	return head, nil
}

// Get retrieves a header by hash.
func (h *RoutingExchange) Get(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	hdr, err := h.core.Get(ctx, hash)
	if err == nil {
		return hdr, nil
	}
	// Fall back to p2p
	return h.p2p.Get(ctx, hash)
}

// GetByHeight retrieves a header by height.
// Uses the availability window to determine which exchange to use.
func (h *RoutingExchange) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return h.core.GetByHeight(ctx, height)
}

// GetRangeByHeight retrieves a range of headers.
// Splits the range based on the availability window cutoff.
func (h *RoutingExchange) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	if from.Height() == to {
		return nil, errors.New("empty range requested")
	}
	if from.Height() > to {
		return nil, errors.New("out of range")
	}

	startHeight := from.Height() + 1
	if startHeight > to {
		return nil, nil
	}

	cutoff, err := h.calculateCutoffHeight(from)
	if err != nil {
		return nil, err
	}

	// All headers are within window - use core
	if startHeight > cutoff {
		log.Debugw("requesting full range from consensus node", "from", from.Height(), "to", to)
		rng, err := h.core.GetRangeByHeight(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("requesting full range from consensus node: %w", err)
		}
		return rng, nil
	}

	// All headers are outside window - use P2P
	if to <= cutoff {
		log.Debugw("requesting full range from p2p network", "from", from.Height(), "to", to)
		rng, err := h.p2p.GetRangeByHeight(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("requesting full range p2p network: %w", err)
		}
		return rng, nil
	}

	// Split: [startHeight, cutoff] from P2P, (cutoff, to] from core
	// cutoff is the last height outside the window (lastPruned), so it should be fetched from P2P
	var headers []*header.ExtendedHeader

	log.Debugw("requesting partial range from p2p network", "from", from, "to", cutoff)
	// Get historical headers from P2P (outside window, including cutoff)
	p2pHeaders, err := h.p2p.GetRangeByHeight(ctx, from, cutoff)
	if err != nil {
		return nil, fmt.Errorf("requesting partial range from p2p network: %w", err)
	}
	headers = append(headers, p2pHeaders...)

	// Get recent headers from core (inside window)
	if len(headers) > 0 {
		from = headers[len(headers)-1]
	}
	log.Debugw("requesting partial range from consensus node", "from", from.Height(), "to", to)
	coreHeaders, err := h.core.GetRangeByHeight(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("requesting partial range from consensus node: %w", err)
	}
	headers = append(headers, coreHeaders...)

	return headers, nil
}

func (h *RoutingExchange) calculateCutoffHeight(from *header.ExtendedHeader) (uint64, error) {
	if h.window == 0 || h.blockTime == 0 {
		return 0, errors.New("window and blockTime can't be 0")
	}

	// Cutoff time: headers with time >= cutoffTime are inside window
	cutoffTime := time.Now().Add(-h.window)

	// If from is already inside window, all subsequent headers are too
	if !from.Time().Before(cutoffTime) {
		return 0, nil
	}

	// Estimate blocks from 'from' until cutoffTime
	timeToCutoff := cutoffTime.Sub(from.Time())
	blocksToCutoff := uint64(timeToCutoff / h.blockTime)
	return from.Height() + blocksToCutoff, nil
}
