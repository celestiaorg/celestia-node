package peers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestPool(t *testing.T) {
	t.Run("add / remove peers", func(t *testing.T) {
		p := newPool()
		peers := []peer.ID{"peer1", "peer1", "peer2", "peer3"}
		// adding same peer twice should not produce copies
		p.add(peers...)
		require.Equal(t, len(peers)-1, p.aliveCount)

		p.remove("peer1", "peer2")
		require.Equal(t, len(peers)-3, p.aliveCount)

		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peers[3], peerID)

		p.remove("peer3")
		p.remove("peer3")
		require.Equal(t, 0, p.aliveCount)
		_, ok = p.tryGet()
		require.False(t, ok)
		fmt.Println(p)
	})

	t.Run("round robin", func(t *testing.T) {
		p := newPool()
		peers := []peer.ID{"peer1", "peer1", "peer2", "peer3"}
		// adding same peer twice should not produce copies
		p.add(peers...)
		require.Equal(t, 3, p.aliveCount)

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
		require.Equal(t, 1, p.aliveCount)

		// pointer should skip removed items until found alive one
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

		p := newPool()
		done := make(chan struct{})

		go func() {
			select {
			case _, ok := <-p.waitNext(shortCtx):
				require.False(t, ok)
				// unlock longCtx waiter by adding new peer
				p.add("peer1")
			case <-longCtx.Done():
				require.NoError(t, longCtx.Err())
			}
		}()

		go func() {
			defer close(done)
			select {
			case peerID := <-p.waitNext(longCtx):
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

	t.Run("next got removed", func(t *testing.T) {
		p := newPool()

		peers := []peer.ID{"peer1", "peer2", "peer3"}
		p.add(peers...)
		p.next = 2
		p.remove(peers[p.next])

		// if previous next was removed, tryGet should iterate until available peer found
		peerID, ok := p.tryGet()
		require.True(t, ok)
		require.Equal(t, peers[0], peerID)
	})

	t.Run("cleanup", func(t *testing.T) {
		p := newPool()
		p.cleanupEnabled = true

		peers := []peer.ID{"peer1", "peer2", "peer3", "peer4", "peer5"}
		p.add(peers...)
		require.Equal(t, len(peers), p.aliveCount)

		// point to last element that will be removed, to check how pointer will be updated
		p.next = len(peers) - 1

		// remove some, but not trigger cleanup yet
		p.remove(peers[3:]...)
		require.Equal(t, len(peers)-2, p.aliveCount)
		require.Equal(t, len(peers), len(p.alive))

		// trigger cleanup
		p.remove(peers[2])
		require.Equal(t, len(peers)-3, p.aliveCount)
		require.Equal(t, len(peers)-3, len(p.alive))
		// next pointer should be updated
		require.Equal(t, 0, p.next)
	})
}
