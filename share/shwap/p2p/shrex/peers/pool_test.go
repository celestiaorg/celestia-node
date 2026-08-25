package peers

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func newTestPool(t *testing.T, peerCooldownTime time.Duration) *pool {
	t.Helper()

	stats, err := newPeerStats()
	require.NoError(t, err)
	return newPool(peerCooldownTime, stats)
}

func TestPool(t *testing.T) {
	t.Run("add / remove peers", func(t *testing.T) {
		p := newTestPool(t, time.Second)

		peers := []peer.ID{"peer1", "peer1", "peer2", "peer3"}
		// adding same peer twice should not produce copies
		p.add(peers...)
		require.Equal(t, len(peers)-1, p.activeCount)

		p.remove("peer1", "peer2")
		require.Equal(t, len(peers)-3, p.activeCount)

		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peers[3], peerID)

		p.remove("peer3")
		p.remove("peer3")
		require.Equal(t, 0, p.activeCount)
		_, ok = p.tryGet()
		require.False(t, ok)
	})

	t.Run("prefers peer with higher throughput", func(t *testing.T) {
		p := newTestPool(t, time.Second)
		p.add("fast", "slow")

		p.stats.updateStats("fast", TransferStats{Bytes: 100 << 20, Duration: time.Second})
		p.stats.updateStats("slow", TransferStats{Bytes: 1 << 10, Duration: time.Second})

		// with two active peers both are always sampled, so the faster one always wins
		for range 10 {
			peerID, ok := p.tryGet()
			require.True(t, ok)
			require.Equal(t, peer.ID("fast"), peerID)
		}
	})

	t.Run("finds second active peer", func(t *testing.T) {
		p := newTestPool(t, time.Second)
		p.add("peer1", "peer2")

		first, idx, ok := p.activeFrom(0)
		require.True(t, ok)

		second, _, ok := p.activeFrom(idx + 1)
		require.True(t, ok)
		require.NotEmpty(t, second)
		require.NotEqual(t, first, second)
	})

	t.Run("unmeasured peer wins against any measured peer", func(t *testing.T) {
		p := newTestPool(t, time.Second)
		p.add("fast", "unmeasured")

		p.stats.updateStats("fast", TransferStats{Bytes: 100 << 20, Duration: time.Second})

		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("unmeasured"), peerID)
	})

	t.Run("does not herd onto the fastest peer", func(t *testing.T) {
		p := newTestPool(t, time.Second)
		p.add("fast", "mid1", "mid2", "mid3")

		p.stats.updateStats("fast", TransferStats{Bytes: 100 << 20, Duration: time.Second})
		for _, mid := range []peer.ID{"mid1", "mid2", "mid3"} {
			p.stats.updateStats(mid, TransferStats{Bytes: 1 << 20, Duration: time.Second})
		}

		got := make(map[peer.ID]int)
		for range 100 {
			peerID, ok := p.tryGet()
			require.True(t, ok)
			got[peerID]++
		}

		// the fastest peer wins every pair it is sampled in, but the pairs it is not
		// sampled in still go to other peers
		require.Greater(t, got["fast"], 0)
		require.Less(t, got["fast"], 100)
	})

	t.Run("removed peers are never returned", func(t *testing.T) {
		p := newTestPool(t, time.Second)

		peers := []peer.ID{"peer1", "peer2", "peer3"}
		p.add(peers...)
		p.remove("peer2", "peer3")
		require.Equal(t, 1, p.activeCount)

		for range 10 {
			peerID, ok := p.tryGet()
			require.True(t, ok)
			require.Equal(t, peer.ID("peer1"), peerID)
		}
	})

	t.Run("wait for peer", func(t *testing.T) {
		timeout := time.Second
		shortCtx, cancel := context.WithTimeout(context.Background(), timeout/10)
		t.Cleanup(cancel)

		longCtx, cancel := context.WithTimeout(context.Background(), timeout)
		t.Cleanup(cancel)

		p := newTestPool(t, time.Second)
		done := make(chan struct{})

		go func() {
			select {
			case <-p.next(shortCtx):
			case <-shortCtx.Done():
				require.Error(t, shortCtx.Err())
				// unlock longCtx waiter by adding new peer
				p.add("peer1")
			}
		}()

		go func() {
			defer close(done)
			select {
			case peerID := <-p.next(longCtx):
				require.Equal(t, peer.ID("peer1"), peerID)
			case <-longCtx.Done():
				require.NoError(t, longCtx.Err())
			}
		}()

		select {
		case <-done:
		case <-longCtx.Done():
			require.NoError(t, longCtx.Err())
		}
	})

	t.Run("cleanup", func(t *testing.T) {
		p := newTestPool(t, time.Second)
		p.cleanupThreshold = 3

		peers := []peer.ID{"peer1", "peer2", "peer3", "peer4", "peer5"}
		p.add(peers...)
		require.Equal(t, len(peers), p.activeCount)

		// remove some, but not trigger cleanup yet
		p.remove(peers[3:]...)
		require.Equal(t, len(peers)-2, p.activeCount)
		require.Equal(t, len(peers), len(p.statuses))

		// trigger cleanup
		p.remove(peers[2])
		require.Equal(t, len(peers)-3, p.activeCount)
		require.Equal(t, len(peers)-3, len(p.statuses))
	})

	t.Run("cooldown blocks get", func(t *testing.T) {
		ttl := time.Second / 10
		p := newTestPool(t, ttl)

		peerID := peer.ID("peer1")
		p.add(peerID)

		_, ok := p.tryGet()
		require.True(t, ok)

		p.putOnCooldown(peerID)
		// item should be unavailable
		_, ok = p.tryGet()
		require.False(t, ok)

		ctx, cancel := context.WithTimeout(context.Background(), ttl*5)
		defer cancel()
		select {
		case <-p.next(ctx):
		case <-ctx.Done():
			t.Fatal("item should be already available")
		}
	})

	t.Run("put on cooldown removed item should be noop", func(t *testing.T) {
		p := newTestPool(t, time.Second)
		p.cleanupThreshold = 3

		peerID := peer.ID("peer1")
		p.add(peerID)

		p.remove(peerID)
		p.cleanup()
		p.putOnCooldown(peerID)

		_, ok := p.tryGet()
		require.False(t, ok)
	})
}
