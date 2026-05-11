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

// Tunables are variables so tests can tighten them without sleeping on
// production-scale durations.
var (
	defaultEndpointAttemptTimeout = 10 * time.Second
	defaultSubscriptionGraceDelay = 500 * time.Millisecond
)

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
// height. For explicit heights, eligible endpoints are returned in
// priority order. Height 0 means "latest" in the CometBFT APIs, so it is
// routed by highest observed tip first, with priority as the tie-breaker.
//
// For explicit heights, an endpoint is eligible iff:
//   - the tracker is currently healthy (not in cooldown), and
//   - the tracker's coverage snapshot includes the height.
func (m *MultiBlockFetcher) routeFor(height int64) []EndpointFetcher {
	if height == 0 {
		return m.routeForLatest()
	}

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

// routeForLatest returns healthy, probed endpoints sorted by highest tip.
// This lets Head()/height-0 callers follow whichever consensus endpoint is
// ahead, instead of rejecting height 0 as being below EarliestHeight.
func (m *MultiBlockFetcher) routeForLatest() []EndpointFetcher {
	type candidate struct {
		ep     EndpointFetcher
		latest int64
	}
	candidates := make([]candidate, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		snap := ep.Tracker.Snapshot()
		if !snap.Healthy || snap.LatestHeight == 0 {
			continue
		}
		candidates = append(candidates, candidate{ep: ep, latest: snap.LatestHeight})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].latest > candidates[j].latest
	})

	out := make([]EndpointFetcher, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.ep)
	}
	return out
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
func (m *MultiBlockFetcher) markResult(ctx context.Context, ep EndpointFetcher, height int64, err error) {
	if err == nil {
		ep.Tracker.MarkSuccess()
		return
	}
	if IsNotFound(err) {
		ep.Tracker.MarkNotFound(height)
		return
	}
	if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		// The caller's request is over; don't penalise an endpoint for work
		// we explicitly abandoned.
		return
	}
	ep.Tracker.MarkFailure(err)
}

func (m *MultiBlockFetcher) noEligibleErr(height int64) error {
	if height == 0 {
		return fmt.Errorf("multi-fetcher: no healthy endpoint has latest status info "+
			"(out of %d configured)", len(m.endpoints))
	}
	return fmt.Errorf("multi-fetcher: no eligible endpoint covers height %d "+
		"(out of %d configured)", height, len(m.endpoints))
}

func (m *MultiBlockFetcher) attemptContext(ctx context.Context, remainingEndpoints int) (context.Context, context.CancelFunc) {
	timeout := defaultEndpointAttemptTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.WithCancel(ctx)
		}
		if remainingEndpoints > 0 {
			perEndpoint := remaining / time.Duration(remainingEndpoints)
			if perEndpoint > 0 && (timeout <= 0 || perEndpoint < timeout) {
				timeout = perEndpoint
			}
		}
	}
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

// GetSignedBlock — primary-preferred fetch within the eligible set.
func (m *MultiBlockFetcher) GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	eligible := m.routeFor(height)
	if len(eligible) == 0 {
		return nil, m.noEligibleErr(height)
	}
	var lastErr error
	for i, ep := range eligible {
		attemptCtx, cancel := m.attemptContext(ctx, len(eligible)-i)
		b, err := ep.Fetcher.GetSignedBlock(attemptCtx, height)
		cancel()
		m.markResult(ctx, ep, height, err)
		if err == nil {
			return b, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
	for i, ep := range eligible {
		attemptCtx, cancel := m.attemptContext(ctx, len(eligible)-i)
		b, err := ep.Fetcher.GetBlock(attemptCtx, height)
		cancel()
		m.markResult(ctx, ep, height, err)
		if err == nil {
			return b, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
	for i, ep := range candidates {
		attemptCtx, cancel := m.attemptContext(ctx, len(candidates)-i)
		b, err := ep.Fetcher.GetBlockByHash(attemptCtx, hash)
		cancel()
		// Use 0 as the height key for tracker bookkeeping since the API
		// doesn't carry one; the tracker accepts NotFound on any height.
		m.markResult(ctx, ep, 0, err)
		if err == nil {
			return b, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
	for i, ep := range eligible {
		attemptCtx, cancel := m.attemptContext(ctx, len(eligible)-i)
		commit, valSet, err := ep.Fetcher.GetBlockInfo(attemptCtx, height)
		cancel()
		m.markResult(ctx, ep, height, err)
		if err == nil {
			return commit, valSet, nil
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
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
	for i, ep := range eligible {
		attemptCtx, cancel := m.attemptContext(ctx, len(eligible)-i)
		c, err := ep.Fetcher.Commit(attemptCtx, height)
		cancel()
		m.markResult(ctx, ep, height, err)
		if err == nil {
			return c, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
	for i, ep := range eligible {
		attemptCtx, cancel := m.attemptContext(ctx, len(eligible)-i)
		v, err := ep.Fetcher.ValidatorSet(attemptCtx, height)
		cancel()
		m.markResult(ctx, ep, height, err)
		if err == nil {
			return v, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
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

	type endpointSubscription struct {
		ep  EndpointFetcher
		sub <-chan SignedBlock
	}

	subs := make([]endpointSubscription, 0, len(m.endpoints))
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
		subs = append(subs, endpointSubscription{ep: ep, sub: sub})
	}

	if len(subs) == 0 {
		close(out)
		return nil, errors.New("multi-fetcher: failed to subscribe on any endpoint")
	}

	primarySubscribed := false
	for _, sub := range subs {
		if sub.ep.Tracker == m.endpoints[0].Tracker {
			primarySubscribed = true
			break
		}
	}

	var wg sync.WaitGroup
	for _, sub := range subs {
		wg.Add(1)
		go func(ep EndpointFetcher, ch <-chan SignedBlock) {
			defer wg.Done()
			m.merge(ctx, ep, ch, out, primarySubscribed)
		}(sub.ep, sub.sub)
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
	primarySubscribed bool,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case b, ok := <-sub:
			if !ok {
				return
			}
			if m.shouldWaitForPrimary(ep, b.Header.Height, primarySubscribed) {
				timer := time.NewTimer(defaultSubscriptionGraceDelay)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return
				}
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

func (m *MultiBlockFetcher) shouldWaitForPrimary(ep EndpointFetcher, height int64, primarySubscribed bool) bool {
	if !primarySubscribed || len(m.endpoints) == 0 || defaultSubscriptionGraceDelay <= 0 {
		return false
	}
	primary := m.endpoints[0]
	if ep.Tracker == primary.Tracker {
		return false
	}
	snap := primary.Tracker.Snapshot()
	return snap.Healthy && snap.Covers(height)
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
