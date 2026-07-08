package peers

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestTimedQueue(t *testing.T) {
	t.Run("push item", func(t *testing.T) {
		peers := []peer.ID{"peer1", "peer2"}
		ttl := time.Second

		popCh := make(chan struct{}, 1)
		queue := newTimedQueue(ttl, func(id peer.ID) {
			go func() {
				require.Contains(t, peers, id)
				popCh <- struct{}{}
			}()
		})
		mock := clock.NewMock()
		queue.clock = mock

		// push first item | global time : 0
		queue.push(peers[0], ttl)
		require.Equal(t, queue.len(), 1)

		// push second item with ttl/2 gap | global time : ttl/2
		mock.Add(ttl / 2)
		queue.push(peers[1], ttl)
		require.Equal(t, queue.len(), 2)

		// advance clock 1 nano sec before first item should expire | global time : ttl - 1
		mock.Add(ttl/2 - 1)
		// check that releaseExpired doesn't remove items
		queue.releaseExpired()
		require.Equal(t, queue.len(), 2)
		// first item should be released after its own timeout | global time : ttl
		mock.Add(1)

		select {
		case <-popCh:
		case <-time.After(ttl):
			t.Fatal("first item is not released")
		}
		require.Equal(t, queue.len(), 1)

		// first item should be released after ttl/2 gap timeout | global time : 3/2*ttl
		mock.Add(ttl / 2)
		select {
		case <-popCh:
		case <-time.After(ttl):
			t.Fatal("second item is not released")
		}
		require.Equal(t, queue.len(), 0)
	})

	t.Run("per-item ttl releases in expiry order", func(t *testing.T) {
		// push a long-ttl item first, then a short-ttl item; the short one must be
		// released first even though it was pushed later.
		short, long := peer.ID("short"), peer.ID("long")

		popCh := make(chan peer.ID, 2)
		queue := newTimedQueue(time.Hour, func(id peer.ID) {
			popCh <- id
		})
		mock := clock.NewMock()
		queue.clock = mock

		queue.push(long, 10*time.Second)
		queue.push(short, time.Second)
		require.Equal(t, 2, queue.len())

		// advance past the short ttl only
		mock.Add(time.Second)
		select {
		case id := <-popCh:
			require.Equal(t, short, id)
		case <-time.After(time.Second):
			t.Fatal("short item not released")
		}
		require.Equal(t, 1, queue.len())

		// advance past the long ttl
		mock.Add(9 * time.Second)
		select {
		case id := <-popCh:
			require.Equal(t, long, id)
		case <-time.After(time.Second):
			t.Fatal("long item not released")
		}
		require.Equal(t, 0, queue.len())
	})
}
