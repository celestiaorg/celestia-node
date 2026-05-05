package core

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiFetcher_Integration_DedupeAcrossRealStreams spins up a real
// test consensus node, dials it twice (two independent *grpc.ClientConn
// instances), wraps each in a BlockFetcher + EndpointTracker, and asserts
// that the MultiBlockFetcher's fan-in subscription dedupes correctly when
// both endpoints deliver the same blocks.
//
// This is the closest the unit-level test suite gets to a real
// multi-endpoint deployment: the routing layer is exercised against
// real gRPC streams and real block hashes, but the chain is shared so
// the happy-path dedupe assertion is meaningful (no spurious mismatches).
func TestMultiFetcher_Integration_DedupeAcrossRealStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	network := startNetwork(t, cfg)
	t.Cleanup(func() { require.NoError(t, network.Stop()) })

	host, port, err := net.SplitHostPort(network.GRPCClient.Target())
	require.NoError(t, err)

	connA := newTestClient(t, host, port)
	connB := newTestClient(t, host, port)

	bfA, err := NewBlockFetcher(connA)
	require.NoError(t, err)
	bfB, err := NewBlockFetcher(connB)
	require.NoError(t, err)

	// Tracker probe intervals are aggressive so coverage info populates
	// fast. The internal initial probe in Run() is what matters most.
	trA := NewEndpointTracker("primary", bfA,
		WithTrackerPriority(0),
		WithTrackerProbeIntervals(500*time.Millisecond, 2*time.Second),
	)
	trB := NewEndpointTracker("secondary", bfB,
		WithTrackerPriority(1),
		WithTrackerProbeIntervals(500*time.Millisecond, 2*time.Second),
	)
	require.NoError(t, trA.Run(ctx))
	require.NoError(t, trB.Run(ctx))
	t.Cleanup(func() { _ = trA.Stop(context.Background()) })
	t.Cleanup(func() { _ = trB.Stop(context.Background()) })

	mf, err := NewMultiBlockFetcher([]EndpointFetcher{
		{Fetcher: bfA, Tracker: trA},
		{Fetcher: bfB, Tracker: trB},
	})
	require.NoError(t, err)

	out, err := mf.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	seen := map[int64]int{}
	deadline := time.After(15 * time.Second)
	for len(seen) < 3 {
		select {
		case b, ok := <-out:
			if !ok {
				t.Fatal("merged channel closed prematurely")
			}
			seen[b.Header.Height]++
		case <-deadline:
			t.Fatalf("timeout waiting for 3 unique heights, got %v", seen)
		}
	}

	// Each height should appear exactly once despite both endpoints
	// delivering it (same chain, same hashes, second-arrival deduped).
	for h, count := range seen {
		assert.Equal(t, 1, count, "height %d delivered %d times — dedupe broken", h, count)
	}
	assert.Equal(t, int64(0), mf.MismatchCount(),
		"unexpected hash mismatches against a single shared chain")

	// Coverage probes should have populated by now.
	snapA := trA.Snapshot()
	snapB := trB.Snapshot()
	assert.Greater(t, snapA.LatestHeight, int64(0), "primary tracker must have probed")
	assert.Greater(t, snapB.LatestHeight, int64(0), "secondary tracker must have probed")
	assert.Equal(t, snapA.ChainID, snapB.ChainID, "both endpoints share the chain ID")
}

// TestMultiFetcher_Integration_PointQueryFallback verifies that if the
// primary endpoint is unhealthy, point queries route to the secondary
// against the same real consensus node.
func TestMultiFetcher_Integration_PointQueryFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	network := startNetwork(t, cfg)
	t.Cleanup(func() { require.NoError(t, network.Stop()) })

	host, port, err := net.SplitHostPort(network.GRPCClient.Target())
	require.NoError(t, err)

	connA := newTestClient(t, host, port)
	connB := newTestClient(t, host, port)

	bfA, err := NewBlockFetcher(connA)
	require.NoError(t, err)
	bfB, err := NewBlockFetcher(connB)
	require.NoError(t, err)

	trA := NewEndpointTracker("primary", bfA,
		WithTrackerPriority(0),
		WithTrackerProbeIntervals(500*time.Millisecond, 2*time.Second),
	)
	trB := NewEndpointTracker("secondary", bfB,
		WithTrackerPriority(1),
		WithTrackerProbeIntervals(500*time.Millisecond, 2*time.Second),
	)
	require.NoError(t, trA.Run(ctx))
	require.NoError(t, trB.Run(ctx))
	t.Cleanup(func() { _ = trA.Stop(context.Background()) })
	t.Cleanup(func() { _ = trB.Stop(context.Background()) })

	mf, err := NewMultiBlockFetcher([]EndpointFetcher{
		{Fetcher: bfA, Tracker: trA},
		{Fetcher: bfB, Tracker: trB},
	})
	require.NoError(t, err)

	// Wait for at least one block to be produced and trackers to see it.
	require.Eventually(t, func() bool {
		return trA.Snapshot().LatestHeight > 0 && trB.Snapshot().LatestHeight > 0
	}, 15*time.Second, 100*time.Millisecond, "trackers did not probe in time")

	// Force the primary into the cooldown state — subsequent queries must
	// route to the secondary.
	trA.MarkFailure(context.DeadlineExceeded) // exempt: caller cancellation
	for range 5 {
		trA.MarkFailure(assertableErr{})
	}
	require.False(t, trA.Healthy(), "primary should be in cooldown after repeated failures")

	height := trB.Snapshot().LatestHeight
	require.Greater(t, height, int64(0))

	b, err := mf.GetSignedBlock(ctx, height)
	require.NoError(t, err)
	require.NotNil(t, b)
	assert.Equal(t, height, b.Header.Height,
		"GetSignedBlock must succeed via the secondary when primary is unhealthy")
}

// assertableErr is a small concrete error type used to drive trackers
// into the unhealthy state. It must NOT be one of the context errors
// (context.Canceled / DeadlineExceeded) — those are exempt from the
// failure budget by design.
type assertableErr struct{}

func (assertableErr) Error() string { return "synthetic transport failure" }
