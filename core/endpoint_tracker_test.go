package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libhead "github.com/celestiaorg/go-header"
)

// fakeFetcher is a programmable Fetcher for tracker tests. It exposes
// hooks for Status responses (success / error) and tracks invocation
// counts so tests can assert on probe cadence.
type fakeFetcher struct {
	mu sync.Mutex

	statusFn  func() (*Status, error)
	statusHit atomic.Int64
}

func newFakeFetcher() *fakeFetcher {
	return &fakeFetcher{}
}

func (f *fakeFetcher) setStatus(fn func() (*Status, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusFn = fn
}

func (f *fakeFetcher) Status(_ context.Context) (*Status, error) {
	f.statusHit.Add(1)
	f.mu.Lock()
	fn := f.statusFn
	f.mu.Unlock()
	if fn == nil {
		return &Status{}, nil
	}
	return fn()
}

// remaining methods unused by EndpointTracker; provided to satisfy Fetcher.
func (f *fakeFetcher) SubscribeNewBlockEvent(_ context.Context) (chan SignedBlock, error) {
	return nil, nil
}

func (f *fakeFetcher) GetBlock(_ context.Context, _ int64) (*SignedBlock, error) {
	return nil, nil
}
func (f *fakeFetcher) GetSignedBlock(_ context.Context, _ int64) (*SignedBlock, error) {
	return nil, nil
}
func (f *fakeFetcher) GetBlockByHash(_ context.Context, _ libhead.Hash) (*types.Block, error) {
	return nil, nil
}

func (f *fakeFetcher) GetBlockInfo(_ context.Context, _ int64) (*types.Commit, *types.ValidatorSet, error) {
	return nil, nil, nil
}
func (f *fakeFetcher) Commit(_ context.Context, _ int64) (*types.Commit, error) { return nil, nil }
func (f *fakeFetcher) ValidatorSet(_ context.Context, _ int64) (*types.ValidatorSet, error) {
	return nil, nil
}
func (f *fakeFetcher) IsSyncing(_ context.Context) (bool, error) { return false, nil }
func (f *fakeFetcher) ChainID(_ context.Context) (string, error) { return "", nil }

var _ Fetcher = (*fakeFetcher)(nil)

func TestEndpointTracker_InitialProbePopulatesState(t *testing.T) {
	f := newFakeFetcher()
	now := time.Unix(1_000_000, 0).UTC()
	f.setStatus(func() (*Status, error) {
		return &Status{
			ChainID:         "private",
			LatestHeight:    100,
			EarliestHeight:  1,
			LatestBlockTime: now.Add(-2 * time.Second),
			CatchingUp:      false,
		}, nil
	})

	tr := NewEndpointTracker("primary", f,
		WithTrackerProbeIntervals(time.Hour, time.Hour),
		withTrackerClock(func() time.Time { return now }),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() {
		_ = tr.Stop(context.Background())
	})

	snap := tr.Snapshot()
	assert.Equal(t, "primary", snap.Name)
	assert.Equal(t, "private", snap.ChainID)
	assert.Equal(t, int64(100), snap.LatestHeight)
	assert.Equal(t, int64(1), snap.EarliestHeight)
	assert.True(t, snap.Healthy)
	assert.False(t, snap.CatchingUp)
	assert.Equal(t, int64(1), f.statusHit.Load(),
		"Run must perform exactly one synchronous probe before returning")
}

func TestEndpointTracker_CoversSemantics(t *testing.T) {
	cases := []struct {
		name     string
		snap     EndpointSnapshot
		height   int64
		expected bool
	}{
		{"unprobed", EndpointSnapshot{}, 5, false},
		{"within range", EndpointSnapshot{EarliestHeight: 1, LatestHeight: 100}, 50, true},
		{"at latest", EndpointSnapshot{EarliestHeight: 1, LatestHeight: 100}, 100, true},
		{"at earliest", EndpointSnapshot{EarliestHeight: 1, LatestHeight: 100}, 1, true},
		{"above latest", EndpointSnapshot{EarliestHeight: 1, LatestHeight: 100}, 101, false},
		{"below earliest", EndpointSnapshot{EarliestHeight: 50, LatestHeight: 100}, 25, false},
		{"earliest unset, archival-style", EndpointSnapshot{EarliestHeight: 0, LatestHeight: 100}, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.snap.Covers(tc.height))
		})
	}
}

func TestEndpointTracker_MarkFailure_TripsCircuitBreaker(t *testing.T) {
	f := newFakeFetcher()
	now := time.Unix(1_000_000, 0).UTC()
	f.setStatus(func() (*Status, error) {
		return &Status{ChainID: "x", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	tr := NewEndpointTracker("p", f,
		WithTrackerProbeIntervals(time.Hour, time.Hour),
		WithTrackerCircuitBreaker(2, time.Minute),
		withTrackerClock(func() time.Time { return now }),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })

	tr.MarkFailure(errors.New("boom"))
	assert.True(t, tr.Healthy(), "single failure must not trip the breaker (threshold=2)")

	tr.MarkFailure(errors.New("boom"))
	assert.False(t, tr.Healthy(), "second failure must trip the breaker")

	// Advance clock past cooldown.
	tr.params.now = func() time.Time { return now.Add(2 * time.Minute) }
	assert.True(t, tr.Healthy(), "tracker must re-enter rotation after cooldown")
}

func TestEndpointTracker_MarkFailure_IgnoresContextCancellation(t *testing.T) {
	f := newFakeFetcher()
	f.setStatus(func() (*Status, error) {
		return &Status{ChainID: "x", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	tr := NewEndpointTracker("p", f,
		WithTrackerProbeIntervals(time.Hour, time.Hour),
		WithTrackerCircuitBreaker(1, time.Minute),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })

	tr.MarkFailure(context.Canceled)
	assert.True(t, tr.Healthy(),
		"caller-side cancellation must not count against the failure budget")
	assert.Equal(t, 0, tr.Snapshot().ConsecFailures)

	tr.MarkFailure(context.DeadlineExceeded)
	assert.False(t, tr.Healthy(),
		"endpoint attempt deadlines must count against the failure budget")
	assert.Equal(t, 1, tr.Snapshot().ConsecFailures)
}

func TestEndpointTracker_MarkSuccess_ResetsBreaker(t *testing.T) {
	f := newFakeFetcher()
	f.setStatus(func() (*Status, error) {
		return &Status{ChainID: "x", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	tr := NewEndpointTracker("p", f,
		WithTrackerProbeIntervals(time.Hour, time.Hour),
		WithTrackerCircuitBreaker(2, time.Minute),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })

	tr.MarkFailure(errors.New("boom"))
	tr.MarkSuccess()
	tr.MarkFailure(errors.New("boom"))
	assert.True(t, tr.Healthy(),
		"MarkSuccess must reset the consecutive-failure counter")
}

func TestEndpointTracker_MarkNotFound_TightensEarliestAndKicksProbe(t *testing.T) {
	f := newFakeFetcher()
	probeCount := atomic.Int64{}
	f.setStatus(func() (*Status, error) {
		probeCount.Add(1)
		return &Status{
			ChainID:        "x",
			LatestHeight:   100,
			EarliestHeight: 10,
		}, nil
	})

	tr := NewEndpointTracker("p", f,
		// Long intervals so any extra probe must come from the kick channel.
		WithTrackerProbeIntervals(time.Hour, time.Hour),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })
	require.Equal(t, int64(1), probeCount.Load(), "exactly one initial probe")

	// NotFound for a height that *was* in the covered range tightens earliest
	// to h+1 and triggers an extra probe.
	tr.MarkNotFound(15)

	require.Eventually(t, func() bool {
		return probeCount.Load() >= 2
	}, time.Second, 5*time.Millisecond, "MarkNotFound must kick a probe")

	// After the kick, the probe re-reports earliest=10, which overrides our
	// optimistic tightening. That's intentional: probes are authoritative.
	snap := tr.Snapshot()
	assert.Equal(t, int64(10), snap.EarliestHeight)

	// NotFound for a height already outside coverage is a no-op.
	tr.MarkNotFound(5)
	assert.Equal(t, int64(10), tr.Snapshot().EarliestHeight)
}

func TestEndpointTracker_ProbeFailure_CountsTowardsBreaker(t *testing.T) {
	f := newFakeFetcher()
	f.setStatus(func() (*Status, error) {
		return nil, errors.New("transport down")
	})
	tr := NewEndpointTracker("p", f,
		WithTrackerProbeIntervals(time.Hour, time.Hour),
		WithTrackerCircuitBreaker(1, time.Minute),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })

	snap := tr.Snapshot()
	assert.False(t, snap.Healthy,
		"a failed initial probe must trip the breaker when threshold=1")
	assert.NotNil(t, snap.LastErr)
}

func TestEndpointTracker_StopIsIdempotent(t *testing.T) {
	f := newFakeFetcher()
	f.setStatus(func() (*Status, error) {
		return &Status{ChainID: "x", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	tr := NewEndpointTracker("p", f, WithTrackerProbeIntervals(time.Hour, time.Hour))
	require.NoError(t, tr.Run(context.Background()))

	require.NoError(t, tr.Stop(context.Background()))
	// Second Stop must not panic or block.
	require.NoError(t, tr.Stop(context.Background()))
}

func TestEndpointTracker_DoubleRunIsRejected(t *testing.T) {
	f := newFakeFetcher()
	f.setStatus(func() (*Status, error) {
		return &Status{ChainID: "x", LatestHeight: 1, EarliestHeight: 1}, nil
	})
	tr := NewEndpointTracker("p", f, WithTrackerProbeIntervals(time.Hour, time.Hour))
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })

	err := tr.Run(context.Background())
	assert.Error(t, err, "starting an already-running tracker must fail")
}

func TestIsNotFound(t *testing.T) {
	assert.False(t, IsNotFound(nil))
	assert.False(t, IsNotFound(errors.New("plain")))
	assert.True(t, IsNotFound(status.Error(codes.NotFound, "no")))
	assert.False(t, IsNotFound(status.Error(codes.Unavailable, "down")))
}

func TestEndpointTracker_BackgroundProbeRefreshesState(t *testing.T) {
	f := newFakeFetcher()
	var height atomic.Int64
	height.Store(100)
	f.setStatus(func() (*Status, error) {
		return &Status{
			ChainID:        "x",
			LatestHeight:   height.Load(),
			EarliestHeight: 1,
		}, nil
	})

	tr := NewEndpointTracker("p", f,
		WithTrackerProbeIntervals(20*time.Millisecond, time.Hour),
	)
	require.NoError(t, tr.Run(context.Background()))
	t.Cleanup(func() { _ = tr.Stop(context.Background()) })

	require.Equal(t, int64(100), tr.Snapshot().LatestHeight)

	height.Store(200)
	require.Eventually(t, func() bool {
		return tr.Snapshot().LatestHeight == 200
	}, time.Second, 5*time.Millisecond, "background tip-probe must refresh latest height")
}
