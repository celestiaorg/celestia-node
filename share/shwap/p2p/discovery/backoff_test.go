package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestBackoff_ConnectPeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], backoff.NewFixedBackoff(time.Minute))
	info := host.InfoFromHost(m.Hosts()[1])
	require.NoError(t, b.Connect(ctx, *info))
}

func TestBackoff_ConnectPeerFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], backoff.NewFixedBackoff(time.Minute))
	info := host.InfoFromHost(m.Hosts()[1])
	require.NoError(t, b.Connect(ctx, *info))

	require.Error(t, b.Connect(ctx, *info))
}

func TestBackoff_ResetBackoffPeriod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], backoff.NewFixedBackoff(time.Minute))
	info := host.InfoFromHost(m.Hosts()[1])
	require.NoError(t, b.Connect(ctx, *info))
	nexttry := b.cacheData[info.ID].nexttry
	b.Backoff(info.ID)
	require.True(t, b.cacheData[info.ID].nexttry.After(nexttry))
}

// TestBackoff_Exponential tests that the delay of a peer grows exponentially
// with every failure and is capped at maxBackoff.
func TestBackoff_Exponential(t *testing.T) {
	b := newBackoffConnector(nil, defaultBackoffFactory)
	id := peer.ID("peer")

	for _, want := range []time.Duration{minBackoff, 2 * minBackoff, 4 * minBackoff, 8 * minBackoff} {
		now := time.Now()
		b.Backoff(id)
		require.InDelta(t, want, b.cacheData[id].nexttry.Sub(now), float64(time.Second))
	}

	for range 10 {
		b.Backoff(id)
	}
	now := time.Now()
	b.Backoff(id)
	require.InDelta(t, maxBackoff, b.cacheData[id].nexttry.Sub(now), float64(time.Second))
}

// TestBackoff_GCKeepsGrowth tests that gc retains the accumulated delay of a peer
// for maxBackoff past its expiry and drops the peer afterwards.
func TestBackoff_GCKeepsGrowth(t *testing.T) {
	b := newBackoffConnector(nil, defaultBackoffFactory)
	id := peer.ID("peer")
	b.Backoff(id)

	expire := func(d time.Duration) {
		b.cacheData[id] = backoffData{nexttry: time.Now().Add(-d), backoff: b.cacheData[id].backoff}
	}

	expire(time.Second)
	b.gc()
	require.Contains(t, b.cacheData, id)

	now := time.Now()
	b.Backoff(id)
	require.InDelta(t, 2*minBackoff, b.cacheData[id].nexttry.Sub(now), float64(time.Second))

	expire(maxBackoff + time.Second)
	b.gc()
	require.NotContains(t, b.cacheData, id)
}

// TestBackoff_ResetOnConnect tests that a successful dial drops the peer's delay
// back to minBackoff.
func TestBackoff_ResetOnConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	b := newBackoffConnector(m.Hosts()[0], defaultBackoffFactory)
	info := host.InfoFromHost(m.Hosts()[1])

	for range 5 {
		b.Backoff(info.ID)
	}
	// clear the delay so that Connect is not rejected by it
	b.cacheData[info.ID] = backoffData{backoff: b.cacheData[info.ID].backoff}

	now := time.Now()
	require.NoError(t, b.Connect(ctx, *info))
	require.InDelta(t, minBackoff, b.cacheData[info.ID].nexttry.Sub(now), float64(time.Second))
}
