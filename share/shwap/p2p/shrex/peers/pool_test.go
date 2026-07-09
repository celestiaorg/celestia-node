package peers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// testPoolParams returns default parameters with an optional cooldown override, used to
// construct pools in tests.
func testPoolParams(cooldown time.Duration) *Parameters {
	p := DefaultParameters()
	if cooldown > 0 {
		p.PeerCooldown = cooldown
		if p.MaxCooldown < cooldown {
			p.MaxCooldown = cooldown
		}
	}
	return p
}

// greedyParams forces deterministic greedy selection by sampling every candidate, so the
// highest-scoring peer is always chosen (P2C with K >= N degenerates to argmax).
func greedyParams() *Parameters {
	p := DefaultParameters()
	p.P2CSampleSize = 1024
	return p
}

func TestPool(t *testing.T) {
	t.Run("add / remove peers", func(t *testing.T) {
		p := newPool(testPoolParams(time.Second))

		peers := []peer.ID{"peer1", "peer1", "peer2", "peer3"}
		// adding same peer twice should not produce copies
		p.add(peers...)
		require.Equal(t, len(peers)-1, p.activeCount)

		p.remove("peer1", "peer2")
		require.Equal(t, len(peers)-3, p.activeCount)

		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("peer3"), peerID)

		p.remove("peer3")
		p.remove("peer3")
		require.Equal(t, 0, p.activeCount)
		_, ok = p.tryGet()
		require.False(t, ok)
	})

	t.Run("tryGet only returns active peers", func(t *testing.T) {
		p := newPool(testPoolParams(time.Second))
		peers := []peer.ID{"peer1", "peer2", "peer3"}
		p.add(peers...)

		p.putOnCooldown("peer2")
		p.remove("peer3")

		// only peer1 is active; repeated gets must always return it
		for i := 0; i < 20; i++ {
			peerID, ok := p.tryGet()
			require.True(t, ok)
			require.Equal(t, peer.ID("peer1"), peerID)
		}
	})

	t.Run("greedy selection prefers higher-quality peer", func(t *testing.T) {
		p := newPool(greedyParams())
		good, bad := peer.ID("good"), peer.ID("bad")
		p.add(good, bad)

		// train: good succeeds fast, bad fails repeatedly
		for i := 0; i < 10; i++ {
			p.recordOutcome(good, true, 100*time.Millisecond, 0)
			p.recordOutcome(bad, false, 0, 0)
			// bad gets cooled on failure; bring it back so it stays selectable
			p.afterCooldown(bad)
		}

		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, good, peerID)
	})

	t.Run("in-flight load spreads selection", func(t *testing.T) {
		p := newPool(greedyParams())
		a, b := peer.ID("a"), peer.ID("b")
		p.add(a, b)

		// both start equal; loading 'a' should make the selector prefer 'b'
		p.acquire(a)
		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, b, peerID)
	})

	t.Run("in-flight accounting via acquire/dec", func(t *testing.T) {
		p := newPool(testPoolParams(time.Second))
		peerID := peer.ID("peer1")
		p.add(peerID)

		p.acquire(peerID)
		p.acquire(peerID)
		require.Equal(t, 2, p.stats[peerID].inFlight)

		p.decInFlight(peerID)
		require.Equal(t, 1, p.stats[peerID].inFlight)

		// never goes negative
		p.decInFlight(peerID)
		p.decInFlight(peerID)
		require.Equal(t, 0, p.stats[peerID].inFlight)
	})

	t.Run("failure puts peer on adaptive cooldown", func(t *testing.T) {
		p := newPool(testPoolParams(time.Second))
		peerID := peer.ID("peer1")
		p.add(peerID)

		p.recordOutcome(peerID, false, 0, 0)
		require.Equal(t, 0, p.activeCount)
		_, ok := p.tryGet()
		require.False(t, ok)
	})

	t.Run("cleanup drops removed peer stats", func(t *testing.T) {
		p := newPool(testPoolParams(time.Second))
		p.cleanupThreshold = 3

		peers := []peer.ID{"peer1", "peer2", "peer3", "peer4", "peer5"}
		p.add(peers...)
		require.Equal(t, len(peers), p.activeCount)
		require.Len(t, p.stats, len(peers))

		// remove some, but do not trigger cleanup yet
		p.remove(peers[3:]...)
		require.Equal(t, len(peers)-2, p.activeCount)
		require.Len(t, p.statuses, len(peers))

		// trigger cleanup
		p.remove(peers[2])
		require.Equal(t, len(peers)-3, p.activeCount)
		require.Len(t, p.statuses, len(peers)-3)
		require.Len(t, p.stats, len(peers)-3)
	})

	t.Run("wait for peer", func(t *testing.T) {
		timeout := time.Second
		shortCtx, cancel := context.WithTimeout(context.Background(), timeout/10)
		t.Cleanup(cancel)

		longCtx, cancel := context.WithTimeout(context.Background(), timeout)
		t.Cleanup(cancel)

		p := newPool(testPoolParams(time.Second))
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

	t.Run("cooldown blocks get", func(t *testing.T) {
		ttl := time.Second / 10
		p := newPool(testPoolParams(ttl))

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
		p := newPool(testPoolParams(time.Second))
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

// TestPoolSelection exercises the Power-of-Two-Choices selection under the real default
// sample size (K=2), which the deterministic greedy tests above intentionally bypass.
func TestPoolSelection(t *testing.T) {
	t.Run("P2C concentrates on the best peer", func(t *testing.T) {
		p := newPool(DefaultParameters()) // K = 2
		best := peer.ID("best")
		peers := []peer.ID{best, "p1", "p2", "p3", "p4"}
		p.add(peers...)

		// seed stats directly; tryGet is read-only so the distribution is stationary
		now := time.Now()
		for _, id := range peers {
			st := p.stats[id]
			st.lastUpdate = now
			if id == best {
				st.successEWMA, st.latencyEWMA = 1, 0.05
			} else {
				st.successEWMA, st.latencyEWMA = 0.2, 2
			}
		}

		const trials = 2000
		count := 0
		for i := 0; i < trials; i++ {
			id, ok := p.tryGet()
			require.True(t, ok)
			if id == best {
				count++
			}
		}
		// With K=2 over 5 peers, the best peer is in the sample ~40% of the time and
		// always wins it. That is far above the uniform share of 1/5 (=20%).
		require.Greater(t, count, trials*30/100, "best peer should be strongly preferred")
	})

	t.Run("load spreads across equal peers", func(t *testing.T) {
		p := newPool(DefaultParameters()) // K = 2, in-flight load penalty
		peers := []peer.ID{"a", "b", "c", "d"}
		p.add(peers...)

		const rounds = 40
		for i := 0; i < rounds; i++ {
			id, ok := p.tryGet()
			require.True(t, ok)
			p.acquire(id) // never released, so load accumulates
		}

		maxLoad := 0
		for _, id := range peers {
			if n := p.stats[id].inFlight; n > maxLoad {
				maxLoad = n
			}
		}
		// P2C + load penalty keeps the busiest peer well below "everything on one".
		require.LessOrEqual(t, maxLoad, 2*rounds/len(peers), "load should be balanced")
	})

	t.Run("prefers a below-cap peer over an at-cap peer", func(t *testing.T) {
		params := DefaultParameters()
		params.InflightCap = 1
		params.P2CSampleSize = 1024 // evaluate all -> deterministic
		p := newPool(params)

		full, free := peer.ID("full"), peer.ID("free")
		p.add(full, free)
		p.acquire(full) // full now at its in-flight cap

		id, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, free, id)
	})

	t.Run("spills to an at-cap peer when it is the only option", func(t *testing.T) {
		params := DefaultParameters()
		params.InflightCap = 2
		p := newPool(params)

		peerID := peer.ID("only")
		p.add(peerID)
		for i := 0; i < params.InflightCap; i++ {
			p.acquire(peerID)
		}
		require.GreaterOrEqual(t, p.stats[peerID].inFlight, params.InflightCap)

		// selection must never block below the server limit: hand the peer out anyway
		id, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peerID, id)
	})

	t.Run("adaptive cooldown escalates end-to-end", func(t *testing.T) {
		params := DefaultParameters()
		params.PeerCooldown = time.Second
		params.CooldownFactor = 2
		params.MaxCooldown = time.Minute
		p := newPool(params)

		mock := clock.NewMock()
		p.cooldown.clock = mock

		peerID := peer.ID("peer1")
		p.add(peerID)

		// first failure -> cooldown of base (1s)
		p.recordOutcome(peerID, false, 0, 0)
		require.Equal(t, 1, p.stats[peerID].consecFails)
		require.Equal(t, 0, p.activeCount)
		mock.Add(time.Second)
		require.Equal(t, 1, p.activeCount, "peer should be released after base cooldown")

		// second consecutive failure -> cooldown of 2*base (2s)
		p.recordOutcome(peerID, false, 0, 0)
		require.Equal(t, 2, p.stats[peerID].consecFails)
		mock.Add(time.Second)
		require.Equal(t, 0, p.activeCount, "peer must still be cooling after only 1s of a 2s cooldown")
		mock.Add(time.Second)
		require.Equal(t, 1, p.activeCount, "peer should be released after the escalated cooldown")
	})
}

// TestPoolConcurrency drives many goroutines through the select/account cycle to catch
// logic races (unbalanced in-flight) that the race detector alone would not surface.
func TestPoolConcurrency(t *testing.T) {
	p := newPool(DefaultParameters())
	peers := []peer.ID{"a", "b", "c", "d", "e"}
	p.add(peers...)

	const (
		workers = 50
		iters   = 200
	)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				id, ok := p.tryGet()
				if !ok {
					continue
				}
				p.acquire(id)
				// keep peers active so selection keeps making progress
				p.recordOutcome(id, true, time.Millisecond, 0)
				p.decInFlight(id)
			}
		}()
	}
	wg.Wait()

	// every acquire was paired with a decrement: no peer should be left in-flight
	for _, id := range peers {
		require.Equal(t, 0, p.stats[id].inFlight, "peer %s left with dangling in-flight", id)
	}
}
