package peers

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestPool(t *testing.T) {
	t.Run("add / remove peers", func(t *testing.T) {
		p := newPool(time.Second)

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

	t.Run("round robin", func(t *testing.T) {
		p := newPool(time.Second)

		peers := []peer.ID{"peer1", "peer1", "peer2", "peer3"}
		// adding same peer twice should not produce copies
		p.add(peers...)
		require.Equal(t, 3, p.activeCount)

		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("peer1"), peerID)

		peerID, ok = p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("peer2"), peerID)

		peerID, ok = p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("peer3"), peerID)

		peerID, ok = p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("peer1"), peerID)

		p.remove("peer2", "peer3")
		require.Equal(t, 1, p.activeCount)

		// pointer should skip removed items until found active one
		peerID, ok = p.tryGet()
		require.True(t, ok)
		require.Equal(t, peer.ID("peer1"), peerID)
	})

	t.Run("wait for peer", func(t *testing.T) {
		timeout := time.Second
		shortCtx, cancel := context.WithTimeout(context.Background(), timeout/10)
		t.Cleanup(cancel)

		longCtx, cancel := context.WithTimeout(context.Background(), timeout)
		t.Cleanup(cancel)

		p := newPool(time.Second)
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

	t.Run("nextIdx got removed", func(t *testing.T) {
		p := newPool(time.Second)

		peers := []peer.ID{"peer1", "peer2", "peer3"}
		p.add(peers...)
		p.nextIdx = 2
		p.remove(peers[p.nextIdx])

		// if previous nextIdx was removed, tryGet should iterate until available peer found
		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peers[0], peerID)
	})

	t.Run("cleanup", func(t *testing.T) {
		p := newPool(time.Second)
		p.cleanupThreshold = 3

		peers := []peer.ID{"peer1", "peer2", "peer3", "peer4", "peer5"}
		p.add(peers...)
		require.Equal(t, len(peers), p.activeCount)

		// point to last element that will be removed, to check how pointer will be updated
		p.nextIdx = len(peers) - 1

		// remove some, but not trigger cleanup yet
		p.remove(peers[3:]...)
		require.Equal(t, len(peers)-2, p.activeCount)
		require.Equal(t, len(peers), len(p.statuses))

		// trigger cleanup
		p.remove(peers[2])
		require.Equal(t, len(peers)-3, p.activeCount)
		require.Equal(t, len(peers)-3, len(p.statuses))

		// nextIdx pointer should be updated after next tryGet
		p.tryGet()
		require.Equal(t, 1, p.nextIdx)
	})

	t.Run("cooldown blocks get", func(t *testing.T) {
		ttl := time.Second / 10
		p := newPool(ttl)

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
		p := newPool(time.Second)
		p.cleanupThreshold = 3

		peerID := peer.ID("peer1")
		p.add(peerID)

		p.remove(peerID)
		p.cleanup()
		p.putOnCooldown(peerID)

		_, ok := p.tryGet()
		require.False(t, ok)
	})

	t.Run("stale cooldown callback", func(t *testing.T) {
		ttl := time.Second
		mock := clock.NewMock()
		p := newPool(ttl)
		p.cooldown.clock = mock

		peerID := peer.ID("peer1")
		p.add(peerID)
		p.putOnCooldown(peerID)
		firstToken := p.cooldownTokens[peerID]

		callbackStarted := make(chan struct{})
		releaseCallback := make(chan struct{})
		callbackDone := make(chan uint64, 1)
		onPop := p.cooldown.onPop
		p.cooldown.onPop = func(id peer.ID, token uint64) {
			if token == firstToken {
				close(callbackStarted)
				<-releaseCallback
			}
			onPop(id, token)
			callbackDone <- token
		}

		advanceDone := make(chan struct{})
		go func() {
			mock.Add(ttl)
			close(advanceDone)
		}()

		select {
		case <-callbackStarted:
		case <-time.After(ttl):
			t.Fatal("first cooldown callback did not start")
		}

		p.remove(peerID)
		require.NotContains(t, p.cooldownTokens, peerID)
		p.add(peerID)
		p.putOnCooldown(peerID)
		secondToken := p.cooldownTokens[peerID]
		require.NotEqual(t, firstToken, secondToken)

		close(releaseCallback)
		select {
		case token := <-callbackDone:
			require.Equal(t, firstToken, token)
		case <-time.After(ttl):
			t.Fatal("first cooldown callback did not finish")
		}
		select {
		case <-advanceDone:
		case <-time.After(ttl):
			t.Fatal("clock advance did not finish")
		}

		_, ok := p.tryGet()
		require.False(t, ok)
		require.Equal(t, 1, p.cooldown.len())

		mock.Add(ttl)
		select {
		case token := <-callbackDone:
			require.Equal(t, secondToken, token)
		case <-time.After(ttl):
			t.Fatal("second cooldown callback did not finish")
		}

		actual, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peerID, actual)
		require.Zero(t, p.cooldown.len())
	})
}
