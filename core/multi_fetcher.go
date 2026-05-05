package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cometbft/cometbft/types"
	lru "github.com/hashicorp/golang-lru/v2"

	libhead "github.com/celestiaorg/go-header"
)

// dedupeRingSize is how many recent heights the MultiBlockFetcher remembers
// for cross-endpoint deduplication and hash-mismatch detection. At a 6s
// blocktime this is ~25 minutes of history — comfortably above any
// reasonable "primary lag, secondary leads" gap.
const dedupeRingSize = 256

// EndpointFetcher pairs a single-endpoint Fetcher with its tracker. The
// tracker drives routing decisions; the Fetcher executes the actual RPC.
type EndpointFetcher struct {
	Fetcher Fetcher
	Tracker *EndpointTracker
}

// MultiBlockFetcher routes Fetcher operations across N consensus
// endpoints. Subscriptions fan-in (latency-optimal); point queries route
// primary-first within the eligible set (consistency-preferred).
//
// MultiBlockFetcher is a drop-in for *BlockFetcher: it implements the
// Fetcher interface so consumers (Listener, Exchange) need no changes.
type MultiBlockFetcher struct {
	// endpoints sorted by priority ascending — endpoints[0] is primary.
	endpoints []EndpointFetcher

	// seenHeights maps height -> first-seen header hash. Used to dedupe
	// cross-endpoint subscription deliveries and to detect hash mismatches.
	seenHeights *lru.Cache[int64, []byte]

	mismatchCount atomic.Int64
}

// NewMultiBlockFetcher builds a multi-endpoint fetcher. The caller is
// responsible for having started each tracker (via Tracker.Run) before
// any traffic is served — the fetcher consults tracker snapshots on every
// routing decision and treats unprobed endpoints as not-yet-eligible.
func NewMultiBlockFetcher(endpoints []EndpointFetcher) (*MultiBlockFetcher, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("multi-fetcher: at least one endpoint required")
	}
	cache, err := lru.New[int64, []byte](dedupeRingSize)
	if err != nil {
		return nil, fmt.Errorf("multi-fetcher: building dedupe ring: %w", err)
	}

	cp := make([]EndpointFetcher, len(endpoints))
	copy(cp, endpoints)
	sort.SliceStable(cp, func(i, j int) bool {
		return cp[i].Tracker.Priority() < cp[j].Tracker.Priority()
	})

	return &MultiBlockFetcher{
		endpoints:   cp,
		seenHeights: cache,
	}, nil
}

// MismatchCount returns the cumulative number of hash mismatches observed
// across endpoint subscriptions. Exposed for metrics and tests.
func (m *MultiBlockFetcher) MismatchCount() int64 { return m.mismatchCount.Load() }

// routeFor returns endpoints eligible to serve a query for the given
// height, in priority order. An endpoint is eligible iff:
//   - the tracker is currently healthy (not in cooldown), and
//   - the tracker's coverage snapshot includes the height.
func (m *MultiBlockFetcher) routeFor(height int64) []EndpointFetcher {
	eligible := make([]EndpointFetcher, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		snap := ep.Tracker.Snapshot()
		if !snap.Healthy {
			continue
		}
		if !snap.Covers(height) {
			continue
		}
		eligible = append(eligible, ep)
	}
	return eligible
}

// healthy returns the subset of endpoints currently healthy, in priority
// order. Used for queries with no height to gate on (e.g. GetBlockByHash).
func (m *MultiBlockFetcher) healthy() []EndpointFetcher {
	out := make([]EndpointFetcher, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		if ep.Tracker.Healthy() {
			out = append(out, ep)
		}
	}
	return out
}

// markResult routes a query result to the right tracker hook. NotFound is
// reactive coverage information, not a failure.
func (m *MultiBlockFetcher) markResult(ep EndpointFetcher, height int64, err error) {
	if err == nil {
		ep.Tracker.MarkSuccess()
		return
	}
	if IsNotFound(err) {
		ep.Tracker.MarkNotFound(height)
		return
	}
	ep.Tracker.MarkFailure(err)
}

func (m *MultiBlockFetcher) noEligibleErr(height int64) error {
	return fmt.Errorf("multi-fetcher: no eligible endpoint covers height %d "+
		"(out of %d configured)", height, len(m.endpoints))
}

// GetSignedBlock — primary-preferred fetch within the eligible set.
func (m *MultiBlockFetcher) GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	eligible := m.routeFor(height)
	if len(eligible) == 0 {
		return nil, m.noEligibleErr(height)
	}
	var lastErr error
	for _, ep := range eligible {
		b, err := ep.Fetcher.GetSignedBlock(ctx, height)
		m.markResult(ep, height, err)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("multi-fetcher: all endpoints failed for height %d: %w", height, lastErr)
}

// GetBlock — same routing as GetSignedBlock.
func (m *MultiBlockFetcher) GetBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	eligible := m.routeFor(height)
	if len(eligible) == 0 {
		return nil, m.noEligibleErr(height)
	}
	var lastErr error
	for _, ep := range eligible {
		b, err := ep.Fetcher.GetBlock(ctx, height)
		m.markResult(ep, height, err)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("multi-fetcher: all endpoints failed for height %d: %w", height, lastErr)
}

// GetBlockByHash has no height to gate on; try all healthy endpoints.
func (m *MultiBlockFetcher) GetBlockByHash(ctx context.Context, hash libhead.Hash) (*types.Block, error) {
	candidates := m.healthy()
	if len(candidates) == 0 {
		return nil, errors.New("multi-fetcher: no healthy endpoints for hash query")
	}
	var lastErr error
	for _, ep := range candidates {
		b, err := ep.Fetcher.GetBlockByHash(ctx, hash)
		// Use 0 as the height key for tracker bookkeeping since the API
		// doesn't carry one; the tracker accepts NotFound on any height.
		m.markResult(ep, 0, err)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("multi-fetcher: all endpoints failed for hash %x: %w", []byte(hash), lastErr)
}

// GetBlockInfo source-binds the (commit, valSet) pair to a single
// endpoint. We never split: if the same endpoint can't serve both, we
// move to the next eligible one.
func (m *MultiBlockFetcher) GetBlockInfo(ctx context.Context, height int64) (*types.Commit, *types.ValidatorSet, error) {
	eligible := m.routeFor(height)
	if len(eligible) == 0 {
		return nil, nil, m.noEligibleErr(height)
	}
	var lastErr error
	for _, ep := range eligible {
		commit, valSet, err := ep.Fetcher.GetBlockInfo(ctx, height)
		m.markResult(ep, height, err)
		if err == nil {
			return commit, valSet, nil
		}
		lastErr = err
	}
	return nil, nil, fmt.Errorf("multi-fetcher: all endpoints failed for height %d: %w", height, lastErr)
}

// Commit — route by coverage. Callers needing source-bound (commit,
// valSet) pairs MUST use GetBlockInfo, not separate Commit + ValidatorSet
// calls (the latter may pick different endpoints).
func (m *MultiBlockFetcher) Commit(ctx context.Context, height int64) (*types.Commit, error) {
	eligible := m.routeFor(height)
	if len(eligible) == 0 {
		return nil, m.noEligibleErr(height)
	}
	var lastErr error
	for _, ep := range eligible {
		c, err := ep.Fetcher.Commit(ctx, height)
		m.markResult(ep, height, err)
		if err == nil {
			return c, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("multi-fetcher: all endpoints failed for height %d: %w", height, lastErr)
}

// ValidatorSet — see Commit's note on source binding.
func (m *MultiBlockFetcher) ValidatorSet(ctx context.Context, height int64) (*types.ValidatorSet, error) {
	eligible := m.routeFor(height)
	if len(eligible) == 0 {
		return nil, m.noEligibleErr(height)
	}
	var lastErr error
	for _, ep := range eligible {
		v, err := ep.Fetcher.ValidatorSet(ctx, height)
		m.markResult(ep, height, err)
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("multi-fetcher: all endpoints failed for height %d: %w", height, lastErr)
}

// Status returns an aggregate view across healthy endpoints. Latest
// height is the maximum across endpoints (so a lagging primary doesn't
// drag us back); earliest is the minimum (so historical coverage reflects
// what *some* endpoint can serve). CatchingUp is false if any healthy
// endpoint is caught up — this matches Listener intent at handleNewSignedBlock.
func (m *MultiBlockFetcher) Status(_ context.Context) (*Status, error) {
	var (
		chainID         string
		latestHeight    int64
		latestBlockTime time.Time
		earliestHeight  int64 = -1
		// catching up is conjunctive: false if ANY endpoint is caught up
		catchingUp = true
		anyHealthy = false
	)
	for _, ep := range m.endpoints {
		snap := ep.Tracker.Snapshot()
		if !snap.Healthy || snap.ChainID == "" {
			continue
		}
		if chainID == "" {
			chainID = snap.ChainID
		} else if snap.ChainID != chainID {
			return nil, fmt.Errorf("multi-fetcher: chain ID mismatch: %q vs %q",
				chainID, snap.ChainID)
		}
		anyHealthy = true
		if snap.LatestHeight > latestHeight {
			latestHeight = snap.LatestHeight
			latestBlockTime = snap.LatestBlockTime
		}
		if earliestHeight < 0 || (snap.EarliestHeight > 0 && snap.EarliestHeight < earliestHeight) {
			earliestHeight = snap.EarliestHeight
		}
		if !snap.CatchingUp {
			catchingUp = false
		}
	}
	if !anyHealthy {
		return nil, errors.New("multi-fetcher: no healthy endpoints with status info")
	}
	if earliestHeight < 0 {
		earliestHeight = 0
	}
	return &Status{
		ChainID:         chainID,
		LatestHeight:    latestHeight,
		EarliestHeight:  earliestHeight,
		LatestBlockTime: latestBlockTime,
		CatchingUp:      catchingUp,
	}, nil
}

// IsSyncing — false if any healthy endpoint is caught up. Errors only if
// no endpoint has a usable snapshot.
func (m *MultiBlockFetcher) IsSyncing(ctx context.Context) (bool, error) {
	s, err := m.Status(ctx)
	if err != nil {
		return false, err
	}
	return s.CatchingUp, nil
}

// ChainID returns the unanimous chain ID across healthy endpoints, or
// errors if endpoints disagree.
func (m *MultiBlockFetcher) ChainID(ctx context.Context) (string, error) {
	s, err := m.Status(ctx)
	if err != nil {
		return "", err
	}
	return s.ChainID, nil
}

// SubscribeNewBlockEvent fans in subscriptions from every healthy
// endpoint. Duplicates (same height, same hash) are silently dropped;
// hash mismatches at the same height are logged loudly, refused (the
// later arrival is dropped), and counted in MismatchCount.
//
// The first arrival per height wins by design: waiting for confirmation
// across endpoints would erase the latency benefit of fan-in. The
// mismatch counter is the operator's signal to investigate.
func (m *MultiBlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (chan SignedBlock, error) {
	out := make(chan SignedBlock, len(m.endpoints))

	var wg sync.WaitGroup
	subscribed := 0
	for _, ep := range m.endpoints {
		// Spawn even for currently-unhealthy endpoints; the underlying
		// BlockFetcher self-retries and will deliver once the endpoint
		// recovers.
		sub, err := ep.Fetcher.SubscribeNewBlockEvent(ctx)
		if err != nil {
			ep.Tracker.MarkFailure(err)
			log.Warnw("multi-fetcher: subscription failed",
				"endpoint", ep.Tracker.Name(), "err", err)
			continue
		}
		subscribed++
		wg.Add(1)
		go func(ep EndpointFetcher, sub <-chan SignedBlock) {
			defer wg.Done()
			m.merge(ctx, ep, sub, out)
		}(ep, sub)
	}

	if subscribed == 0 {
		close(out)
		return nil, errors.New("multi-fetcher: failed to subscribe on any endpoint")
	}

	// Close the output channel once all merge goroutines have exited.
	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
}

func (m *MultiBlockFetcher) merge(
	ctx context.Context,
	ep EndpointFetcher,
	sub <-chan SignedBlock,
	out chan<- SignedBlock,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case b, ok := <-sub:
			if !ok {
				return
			}
			if !m.acceptBlock(ep, b) {
				continue
			}
			select {
			case out <- b:
			case <-ctx.Done():
				return
			}
		}
	}
}

// acceptBlock decides whether an inbound block from endpoint ep should be
// forwarded to the merged subscription channel. Returns false if the
// height has already been forwarded (duplicate from a different endpoint)
// or if the hash disagrees with the previously seen one for this height
// (Byzantine / forked endpoint signal).
func (m *MultiBlockFetcher) acceptBlock(ep EndpointFetcher, b SignedBlock) bool {
	height := b.Header.Height
	hash := []byte(b.Header.Hash())

	if firstHash, seen := m.seenHeights.Get(height); seen {
		if !bytes.Equal(firstHash, hash) {
			m.mismatchCount.Add(1)
			log.Errorw("multi-fetcher: hash mismatch across endpoints — refusing block",
				"height", height,
				"first_hash", hex.EncodeToString(firstHash),
				"this_hash", hex.EncodeToString(hash),
				"this_endpoint", ep.Tracker.Name(),
			)
			return false
		}
		// duplicate from a slower endpoint; expected
		return false
	}
	m.seenHeights.Add(height, hash)
	ep.Tracker.MarkSuccess()
	return true
}

// compile-time assertion: MultiBlockFetcher must satisfy Fetcher.
var _ Fetcher = (*MultiBlockFetcher)(nil)
