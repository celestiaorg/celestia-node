package core

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

// RoutingExchangeOption is a functional option for configuring RoutingExchange.
type RoutingExchangeOption func(*RoutingExchange)

// WithStorageWindow sets the storage window duration for cutoff calculation.
func WithStorageWindow(window time.Duration) RoutingExchangeOption {
	return func(h *RoutingExchange) {
		h.window = window
	}
}

// WithBlockTime sets the block time for height estimation.
func WithBlockTime(blockTime time.Duration) RoutingExchangeOption {
	return func(h *RoutingExchange) {
		h.blockTime = blockTime
	}
}

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
	// cutoffHeight is the cached height at the edge of the availability window.
	// Updated on every Head() call.
	cutoffHeight uint64
}

// NewRoutingExchange creates a new RoutingExchange that wraps core and P2P exchanges.
func NewRoutingExchange(
	coreEx libhead.Exchange[*header.ExtendedHeader],
	p2pEx libhead.Exchange[*header.ExtendedHeader],
	opts ...RoutingExchangeOption,
) *RoutingExchange {
	h := &RoutingExchange{
		core: coreEx,
		p2p:  p2pEx,
	}
	for _, opt := range opts {
		opt(h)
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
		return nil, err
	}

	// Update cached cutoff height
	h.updateCutoffHeight(head.Height())
	return head, nil
}

// updateCutoffHeight recalculates and caches the cutoff height based on head height.
func (h *RoutingExchange) updateCutoffHeight(headHeight uint64) {
	if h.window == 0 || h.blockTime == 0 {
		return
	}

	blocksInWindow := uint64(h.window / h.blockTime)
	if headHeight <= blocksInWindow {
		atomic.StoreUint64(&h.cutoffHeight, 0)
		return
	}
	headHeight -= blocksInWindow
	atomic.StoreUint64(&h.cutoffHeight, headHeight)
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
	cutoff := h.windowCutoffHeight()
	if height > cutoff {
		return h.core.GetByHeight(ctx, height)
	}
	return h.p2p.GetByHeight(ctx, height)
}

// GetRangeByHeight retrieves a range of headers.
// Splits the range based on the availability window cutoff.
func (h *RoutingExchange) GetRangeByHeight(
	ctx context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	if from.Height() == to {
		return []*header.ExtendedHeader{}, errors.New("empty range requested")
	}
	if from.Height() > to {
		return []*header.ExtendedHeader{}, errors.New("out of range")
	}

	startHeight := from.Height() + 1
	if startHeight > to {
		return nil, nil
	}

	cutoff := h.windowCutoffHeight()

	// All headers are within window - use core
	if startHeight > cutoff {
		return h.core.GetRangeByHeight(ctx, from, to)
	}

	// All headers are outside window - use P2P
	if to <= cutoff {
		return h.p2p.GetRangeByHeight(ctx, from, to)
	}

	// Split: [startHeight, cutoff] from P2P, (cutoff, to] from core
	// cutoff is the last height outside the window (lastPruned), so it should be fetched from P2P
	var headers []*header.ExtendedHeader

	// Get historical headers from P2P (outside window, including cutoff)
	p2pHeaders, err := h.p2p.GetRangeByHeight(ctx, from, cutoff)
	if err != nil {
		return nil, err
	}
	headers = append(headers, p2pHeaders...)

	// Get recent headers from core (inside window)
	if len(headers) > 0 {
		from = headers[len(headers)-1]
	}
	coreHeaders, err := h.core.GetRangeByHeight(ctx, from, to)
	if err != nil {
		return nil, err
	}
	headers = append(headers, coreHeaders...)

	return headers, nil
}

// windowCutoffHeight returns the cached height at the edge of the availability window.
// The cutoff is updated on every Head() call.
func (h *RoutingExchange) windowCutoffHeight() uint64 {
	return atomic.LoadUint64(&h.cutoffHeight)
}
