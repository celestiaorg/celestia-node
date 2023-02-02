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
		p.cleanupDisabled = true

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
		fmt.Println(p)
	})

	t.Run("round robin", func(t *testing.T) {
		p := newPool()
		p.cleanupDisabled = true

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

		p := newPool()
		p.cleanupDisabled = true
		done := make(chan struct{})

		go func() {
			select {
			case <-p.getNext(shortCtx):
			case <-shortCtx.Done():
				require.Error(t, shortCtx.Err())
				// unlock longCtx waiter by adding new peer
				p.add("peer1")
			}
		}()

		go func() {
			defer close(done)
			select {
			case peerID := <-p.getNext(longCtx):
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
		p.cleanupDisabled = true

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

		peers := []peer.ID{"peer1", "peer2", "peer3", "peer4", "peer5"}
		p.add(peers...)
		require.Equal(t, len(peers), p.activeCount)

		// point to last element that will be removed, to check how pointer will be updated
		p.next = len(peers) - 1

		// remove some, but not trigger cleanup yet
		p.remove(peers[3:]...)
		require.Equal(t, len(peers)-2, p.activeCount)
		require.Equal(t, len(peers), len(p.active))

		// trigger cleanup
		p.remove(peers[2])
		require.Equal(t, len(peers)-3, p.activeCount)
		require.Equal(t, len(peers)-3, len(p.active))
		// next pointer should be updated
		require.Equal(t, 0, p.next)
	})
}
