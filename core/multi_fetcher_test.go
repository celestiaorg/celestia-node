package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fullFakeFetcher extends fakeFetcher with programmable point-query
// behaviour. The point-query hooks return errors or stub blocks based on
// per-test wiring, and we count invocations so we can assert on routing
// decisions.
type fullFakeFetcher struct {
	fakeFetcher

	// programmable per-method hooks
	getSignedBlockFn    func(height int64) (*SignedBlock, error)
	getSignedBlockCtxFn func(ctx context.Context, height int64) (*SignedBlock, error)
	getBlockInfoFn      func(height int64) (*types.Commit, *types.ValidatorSet, error)
	subFn               func(ctx context.Context) (chan SignedBlock, error)

	// invocation counters
	getSignedBlockHits atomic.Int64
	getBlockInfoHits   atomic.Int64
	subscribeHits      atomic.Int64
}

func (f *fullFakeFetcher) GetSignedBlock(ctx context.Context, height int64) (*SignedBlock, error) {
	f.getSignedBlockHits.Add(1)
	if f.getSignedBlockCtxFn != nil {
		return f.getSignedBlockCtxFn(ctx, height)
	}
	if f.getSignedBlockFn == nil {
		return nil, errors.New("not configured")
	}
	return f.getSignedBlockFn(height)
}

func (f *fullFakeFetcher) GetBlockInfo(_ context.Context, height int64) (*types.Commit, *types.ValidatorSet, error) {
	f.getBlockInfoHits.Add(1)
	if f.getBlockInfoFn == nil {
		return nil, nil, errors.New("not configured")
	}
	return f.getBlockInfoFn(height)
}

func (f *fullFakeFetcher) SubscribeNewBlockEvent(ctx context.Context) (chan SignedBlock, error) {
	f.subscribeHits.Add(1)
	if f.subFn == nil {
		return nil, errors.New("not configured")
	}
	return f.subFn(ctx)
}

// stubBlock builds a minimal SignedBlock for routing-layer tests.
// types.Header.Hash() returns nil unless ValidatorsHash is non-empty, so
// we set it to a sentinel byte slice (32 bytes is the standard size) to
// force Hash() to produce a real digest. ChainID still varies the digest,
// so two stubs with different chainIDs produce different hashes — that's
// what the mismatch test relies on.
func stubBlock(height int64, chainID string) SignedBlock {
	sentinel := make([]byte, 32) // zero-filled is fine; Hash() just needs non-empty
	h := &types.Header{
		ChainID:            chainID,
		Height:             height,
		Time:               time.Unix(1_000_000+height, 0).UTC(),
		ValidatorsHash:     sentinel,
		NextValidatorsHash: sentinel,
		ConsensusHash:      sentinel,
		AppHash:            sentinel,
		LastResultsHash:    sentinel,
		EvidenceHash:       sentinel,
		LastCommitHash:     sentinel,
		DataHash:           sentinel,
	}
	return SignedBlock{Header: h, Data: &types.Data{}, Commit: &types.Commit{}, ValidatorSet: &types.ValidatorSet{}}
}

// makeRouter wires up N trackers (with priorities 0..N-1) and starts each
// with a synthetic Status snapshot so the trackers report Healthy and
// cover the requested ranges. Probe intervals are stretched out so no
// real probe fires during the test.
func makeRouter(t *testing.T, name string, fakes ...*fullFakeFetcher) *MultiBlockFetcher {
	t.Helper()
	endpoints := make([]EndpointFetcher, 0, len(fakes))
	for i, f := range fakes {
		// Default Status: chainID "test", earliest=1, latest=10_000.
		// Tests can override per-fake via setStatus.
		if f.statusFn == nil {
			f.setStatus(func() (*Status, error) {
				return &Status{
					ChainID:        "test",
					EarliestHeight: 1,
					LatestHeight:   10_000,
				}, nil
			})
		}
		tr := NewEndpointTracker(name+"-"+string(rune('a'+i)), f,
			WithTrackerPriority(i),
			WithTrackerProbeIntervals(time.Hour, time.Hour),
		)
		require.NoError(t, tr.Run(context.Background()))
		t.Cleanup(func() { _ = tr.Stop(context.Background()) })
		endpoints = append(endpoints, EndpointFetcher{Fetcher: f, Tracker: tr})
	}
	mf, err := NewMultiBlockFetcher(endpoints)
	require.NoError(t, err)
	return mf
}

func TestMultiFetcher_PrimaryPreferredWhenBothCover(t *testing.T) {
	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		b := stubBlock(h, "test")
		return &b, nil
	}
	secondary.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		t.Fatalf("secondary must not be queried when primary covers and succeeds")
		return nil, nil
	}

	mf := makeRouter(t, "x", primary, secondary)
	b, err := mf.GetSignedBlock(context.Background(), 50)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, int64(50), b.Header.Height)
	assert.Equal(t, int64(1), primary.getSignedBlockHits.Load())
	assert.Equal(t, int64(0), secondary.getSignedBlockHits.Load())
}

func TestMultiFetcher_LatestRoutesToHighestHealthyTip(t *testing.T) {
	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.setStatus(func() (*Status, error) {
		return &Status{ChainID: "test", EarliestHeight: 1, LatestHeight: 90}, nil
	})
	secondary.setStatus(func() (*Status, error) {
		return &Status{ChainID: "test", EarliestHeight: 1, LatestHeight: 100}, nil
	})
	primary.getSignedBlockFn = func(_ int64) (*SignedBlock, error) {
		t.Fatalf("lagging primary must not serve a latest-height query")
		return nil, nil
	}
	secondary.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		require.Equal(t, int64(0), h)
		b := stubBlock(100, "test")
		return &b, nil
	}

	mf := makeRouter(t, "x", primary, secondary)
	b, err := mf.GetSignedBlock(context.Background(), 0)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, int64(100), b.Header.Height)
	assert.Equal(t, int64(0), primary.getSignedBlockHits.Load())
	assert.Equal(t, int64(1), secondary.getSignedBlockHits.Load())
}

func TestMultiFetcher_FallsBackOnNotFound(t *testing.T) {
	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.getSignedBlockFn = func(_ int64) (*SignedBlock, error) {
		return nil, status.Error(codes.NotFound, "pruned")
	}
	secondary.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		b := stubBlock(h, "test")
		return &b, nil
	}

	mf := makeRouter(t, "x", primary, secondary)
	b, err := mf.GetSignedBlock(context.Background(), 50)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, int64(1), primary.getSignedBlockHits.Load())
	assert.Equal(t, int64(1), secondary.getSignedBlockHits.Load())
	// NotFound is not a failure — primary must still be Healthy.
	assert.True(t, mf.endpoints[0].Tracker.Healthy(),
		"NotFound must not trip the breaker on the primary")
}

func TestMultiFetcher_PerEndpointTimeoutLeavesFallbackBudget(t *testing.T) {
	oldTimeout := defaultEndpointAttemptTimeout
	defaultEndpointAttemptTimeout = 20 * time.Millisecond
	t.Cleanup(func() { defaultEndpointAttemptTimeout = oldTimeout })

	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.getSignedBlockCtxFn = func(ctx context.Context, _ int64) (*SignedBlock, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	secondary.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		b := stubBlock(h, "test")
		return &b, nil
	}

	mf := makeRouter(t, "x", primary, secondary)
	b, err := mf.GetSignedBlock(context.Background(), 50)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, int64(1), primary.getSignedBlockHits.Load())
	assert.Equal(t, int64(1), secondary.getSignedBlockHits.Load())
	assert.Equal(t, 1, mf.endpoints[0].Tracker.Snapshot().ConsecFailures,
		"endpoint attempt timeout should count against the slow primary")
}

func TestMultiFetcher_SkipsUncoveredEndpointSilently(t *testing.T) {
	pruned := &fullFakeFetcher{}
	pruned.setStatus(func() (*Status, error) {
		// Only covers heights >= 1000.
		return &Status{ChainID: "test", EarliestHeight: 1000, LatestHeight: 10_000}, nil
	})
	pruned.getSignedBlockFn = func(_ int64) (*SignedBlock, error) {
		t.Fatalf("uncovered endpoint must not be queried")
		return nil, nil
	}
	archival := &fullFakeFetcher{}
	archival.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		b := stubBlock(h, "test")
		return &b, nil
	}

	mf := makeRouter(t, "x", pruned, archival)
	_, err := mf.GetSignedBlock(context.Background(), 50)
	require.NoError(t, err)
	// pruned must not see this query and must not be marked failed.
	assert.True(t, mf.endpoints[0].Tracker.Healthy())
	assert.Equal(t, int64(0), pruned.getSignedBlockHits.Load())
	assert.Equal(t, int64(1), archival.getSignedBlockHits.Load())
}

func TestMultiFetcher_NoEligibleEndpointReturnsTypedError(t *testing.T) {
	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.setStatus(func() (*Status, error) {
		return &Status{ChainID: "test", EarliestHeight: 1000, LatestHeight: 5000}, nil
	})
	b.setStatus(func() (*Status, error) {
		return &Status{ChainID: "test", EarliestHeight: 1000, LatestHeight: 5000}, nil
	})

	mf := makeRouter(t, "x", a, b)
	_, err := mf.GetSignedBlock(context.Background(), 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no eligible endpoint")
}

func TestMultiFetcher_GetBlockInfoSourceBound(t *testing.T) {
	// primary returns success on commit but fails on valSet — but since we
	// ask via GetBlockInfo, the source is bound: primary returns BOTH or
	// NEITHER, never partial. Here primary fails the whole call, secondary
	// succeeds the whole call.
	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.getBlockInfoFn = func(_ int64) (*types.Commit, *types.ValidatorSet, error) {
		return nil, nil, status.Error(codes.NotFound, "pruned")
	}
	secCommit := &types.Commit{Height: 50}
	secValSet := &types.ValidatorSet{}
	secondary.getBlockInfoFn = func(_ int64) (*types.Commit, *types.ValidatorSet, error) {
		return secCommit, secValSet, nil
	}

	mf := makeRouter(t, "x", primary, secondary)
	c, v, err := mf.GetBlockInfo(context.Background(), 50)
	require.NoError(t, err)
	assert.Same(t, secCommit, c)
	assert.Same(t, secValSet, v)
}

func TestMultiFetcher_FailureCountsAndCircuitBreaks(t *testing.T) {
	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.getSignedBlockFn = func(_ int64) (*SignedBlock, error) {
		return nil, errors.New("transport down")
	}
	secondary.getSignedBlockFn = func(h int64) (*SignedBlock, error) {
		b := stubBlock(h, "test")
		return &b, nil
	}

	endpoints := []EndpointFetcher{}
	for i, f := range []*fullFakeFetcher{primary, secondary} {
		f := f
		f.setStatus(func() (*Status, error) {
			return &Status{ChainID: "test", EarliestHeight: 1, LatestHeight: 10_000}, nil
		})
		tr := NewEndpointTracker("ep", f,
			WithTrackerPriority(i),
			WithTrackerProbeIntervals(time.Hour, time.Hour),
			WithTrackerCircuitBreaker(2, time.Minute), // tight breaker for the test
		)
		require.NoError(t, tr.Run(context.Background()))
		t.Cleanup(func() { _ = tr.Stop(context.Background()) })
		endpoints = append(endpoints, EndpointFetcher{Fetcher: f, Tracker: tr})
	}
	mf, err := NewMultiBlockFetcher(endpoints)
	require.NoError(t, err)

	// Two failed queries on primary should trip the breaker.
	_, _ = mf.GetSignedBlock(context.Background(), 50)
	_, _ = mf.GetSignedBlock(context.Background(), 51)
	assert.False(t, mf.endpoints[0].Tracker.Healthy(),
		"primary must be unhealthy after threshold consecutive failures")

	// Subsequent calls should skip primary entirely.
	primaryHitsBefore := primary.getSignedBlockHits.Load()
	_, err = mf.GetSignedBlock(context.Background(), 52)
	require.NoError(t, err)
	assert.Equal(t, primaryHitsBefore, primary.getSignedBlockHits.Load(),
		"unhealthy primary must be skipped")
}

func TestMultiFetcher_StatusAggregatesAcrossEndpoints(t *testing.T) {
	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.setStatus(func() (*Status, error) {
		return &Status{
			ChainID:        "test",
			LatestHeight:   90,
			EarliestHeight: 1,
			CatchingUp:     true, // a is still catching up
		}, nil
	})
	b.setStatus(func() (*Status, error) {
		return &Status{
			ChainID:        "test",
			LatestHeight:   100, // b is ahead
			EarliestHeight: 50,  // b is pruned
			CatchingUp:     false,
		}, nil
	})
	mf := makeRouter(t, "x", a, b)

	s, err := mf.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(100), s.LatestHeight, "max latest across endpoints")
	assert.Equal(t, int64(1), s.EarliestHeight, "min earliest across endpoints")
	assert.False(t, s.CatchingUp, "any-caught-up wins")
}

func TestMultiFetcher_StatusErrorsOnChainIDDisagreement(t *testing.T) {
	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.setStatus(func() (*Status, error) {
		return &Status{ChainID: "mainnet", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	b.setStatus(func() (*Status, error) {
		return &Status{ChainID: "mocha", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	mf := makeRouter(t, "x", a, b)

	_, err := mf.Status(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain ID mismatch")
}

func TestMultiFetcher_SubscribeFanInDedupes(t *testing.T) {
	// Both endpoints push the same heights with the same hashes; we should
	// only see each height once in the merged channel.
	chA := make(chan SignedBlock, 4)
	chB := make(chan SignedBlock, 4)

	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.subFn = func(ctx context.Context) (chan SignedBlock, error) { return chA, nil }
	b.subFn = func(ctx context.Context) (chan SignedBlock, error) { return chB, nil }

	mf := makeRouter(t, "x", a, b)
	out, err := mf.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	// push heights 1..3 from both endpoints with identical headers
	for h := int64(1); h <= 3; h++ {
		chA <- stubBlock(h, "test")
		chB <- stubBlock(h, "test")
	}
	close(chA)
	close(chB)

	seen := map[int64]int{}
	timeout := time.After(2 * time.Second)
	for got := 0; got < 3; {
		select {
		case b, ok := <-out:
			if !ok {
				break
			}
			seen[b.Header.Height]++
			got++
		case <-timeout:
			t.Fatalf("timeout: only got %d unique heights, seen=%v", got, seen)
		}
	}
	assert.Equal(t, map[int64]int{1: 1, 2: 1, 3: 1}, seen)
	assert.Equal(t, int64(0), mf.MismatchCount(), "no mismatches expected")
}

func TestMultiFetcher_SubscribeRefusesHashMismatch(t *testing.T) {
	chA := make(chan SignedBlock, 2)
	chB := make(chan SignedBlock, 2)

	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.subFn = func(ctx context.Context) (chan SignedBlock, error) { return chA, nil }
	b.subFn = func(ctx context.Context) (chan SignedBlock, error) { return chB, nil }

	mf := makeRouter(t, "x", a, b)
	out, err := mf.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	// Same height, different ChainID → different header hash → mismatch.
	chA <- stubBlock(42, "chainA")
	// Wait until the first one is forwarded and recorded; otherwise B's
	// arrival could win the race.
	first := <-out
	require.Equal(t, int64(42), first.Header.Height)

	chB <- stubBlock(42, "chainB-fork") // different chain ⇒ different hash
	close(chA)
	close(chB)

	// Drain until output is closed; we must NOT see height 42 a second time.
	more := false
	for b := range out {
		if b.Header.Height == 42 {
			more = true
		}
	}
	assert.False(t, more, "second-arrival mismatching block must be refused")
	require.Eventually(t, func() bool { return mf.MismatchCount() >= 1 },
		time.Second, 10*time.Millisecond)
}

func TestMultiFetcher_SubscribeWaitsForPrimaryBeforeSecondary(t *testing.T) {
	oldDelay := defaultSubscriptionGraceDelay
	defaultSubscriptionGraceDelay = 50 * time.Millisecond
	t.Cleanup(func() { defaultSubscriptionGraceDelay = oldDelay })

	chPrimary := make(chan SignedBlock, 2)
	chSecondary := make(chan SignedBlock, 2)
	primary, secondary := &fullFakeFetcher{}, &fullFakeFetcher{}
	primary.subFn = func(context.Context) (chan SignedBlock, error) { return chPrimary, nil }
	secondary.subFn = func(context.Context) (chan SignedBlock, error) { return chSecondary, nil }

	mf := makeRouter(t, "x", primary, secondary)
	out, err := mf.SubscribeNewBlockEvent(context.Background())
	require.NoError(t, err)

	chSecondary <- stubBlock(42, "secondary-fork")
	select {
	case b := <-out:
		t.Fatalf("secondary block forwarded before primary grace elapsed: height=%d chain=%s",
			b.Header.Height, b.Header.ChainID)
	case <-time.After(10 * time.Millisecond):
	}

	chPrimary <- stubBlock(42, "primary")
	first := <-out
	require.Equal(t, int64(42), first.Header.Height)
	assert.Equal(t, "primary", first.Header.ChainID)

	close(chPrimary)
	close(chSecondary)
	require.Eventually(t, func() bool { return mf.MismatchCount() >= 1 },
		time.Second, 10*time.Millisecond)
}

func TestMultiFetcher_SubscribeFailsOnAllEndpointsDown(t *testing.T) {
	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.subFn = func(_ context.Context) (chan SignedBlock, error) { return nil, errors.New("a down") }
	b.subFn = func(_ context.Context) (chan SignedBlock, error) { return nil, errors.New("b down") }

	mf := makeRouter(t, "x", a, b)
	_, err := mf.SubscribeNewBlockEvent(context.Background())
	require.Error(t, err)
}

func TestMultiFetcher_NewRequiresAtLeastOneEndpoint(t *testing.T) {
	_, err := NewMultiBlockFetcher(nil)
	require.Error(t, err)
}

func TestMultiFetcher_PrioritySorting(t *testing.T) {
	// Build with deliberately scrambled priorities and confirm the
	// internal slice ends up primary-first.
	a, b, c := &fullFakeFetcher{}, &fullFakeFetcher{}, &fullFakeFetcher{}
	for _, f := range []*fullFakeFetcher{a, b, c} {
		f := f
		f.setStatus(func() (*Status, error) {
			return &Status{ChainID: "test", EarliestHeight: 1, LatestHeight: 10}, nil
		})
	}

	makeTracker := func(prio int, f Fetcher) *EndpointTracker {
		tr := NewEndpointTracker("ep", f,
			WithTrackerPriority(prio),
			WithTrackerProbeIntervals(time.Hour, time.Hour),
		)
		require.NoError(t, tr.Run(context.Background()))
		t.Cleanup(func() { _ = tr.Stop(context.Background()) })
		return tr
	}

	endpoints := []EndpointFetcher{
		{Fetcher: c, Tracker: makeTracker(2, c)},
		{Fetcher: a, Tracker: makeTracker(0, a)},
		{Fetcher: b, Tracker: makeTracker(1, b)},
	}
	mf, err := NewMultiBlockFetcher(endpoints)
	require.NoError(t, err)
	assert.Equal(t, 0, mf.endpoints[0].Tracker.Priority())
	assert.Equal(t, 1, mf.endpoints[1].Tracker.Priority())
	assert.Equal(t, 2, mf.endpoints[2].Tracker.Priority())
}

// TestMultiFetcher_GoroutineCleanup makes sure the merge goroutines exit
// when the parent context is cancelled — otherwise we'd leak per
// SubscribeNewBlockEvent call.
func TestMultiFetcher_GoroutineCleanup(t *testing.T) {
	chA := make(chan SignedBlock, 1)
	chB := make(chan SignedBlock, 1)
	a, b := &fullFakeFetcher{}, &fullFakeFetcher{}
	a.subFn = func(_ context.Context) (chan SignedBlock, error) { return chA, nil }
	b.subFn = func(_ context.Context) (chan SignedBlock, error) { return chB, nil }

	mf := makeRouter(t, "x", a, b)
	ctx, cancel := context.WithCancel(context.Background())
	out, err := mf.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	cancel()
	// Closing the underlying channels — the merge goroutines exit on ctx.Done
	// and the closer goroutine then closes `out`. Without ctx-cancellation
	// they'd block forever.
	close(chA)
	close(chB)

	done := make(chan struct{})
	go func() {
		for range out {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("merge goroutines did not exit after ctx cancellation")
	}
}
