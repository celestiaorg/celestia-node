package peers

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestTimedQueue(t *testing.T) {
	t.Run("push item", func(t *testing.T) {
		peers := []peer.ID{"peer1", "peer2"}
		ttl := time.Second

		popCh := make(chan struct{})
		queue := newTimedQueue(ttl, func(id peer.ID) {
			require.Contains(t, peers, id)
			popCh <- struct{}{}
		})
		clock := clockwork.NewFakeClock()
		queue.clock = clock

		// push first item | global time : 0
		queue.push(peers[0])
		require.Equal(t, queue.len(), 1)

		// push second item with ttl/2 gap | global time : ttl/2
		clock.Advance(ttl / 2)
		queue.push(peers[1])
		require.Equal(t, queue.len(), 2)

		// advance clock 1 nano sec before first item should expire | global time : ttl - 1
		clock.Advance(ttl/2 - 1)
		// check that releaseExpired doesn't remove items
		queue.releaseExpired()
		require.Equal(t, queue.len(), 2)

		// first item should be released after its own timeout | global time : ttl
		clock.Advance(1)
		select {
		case <-popCh:
		case <-time.After(ttl):
			t.Fatal("first item is not released")
		}
		require.Equal(t, queue.len(), 1)

		// first item should be released after ttl/2 gap timeout | global time : 3/2*ttl
		clock.Advance(ttl / 2)
		select {
		case <-popCh:
		case <-time.After(ttl):
			t.Fatal("second item is not released")
		}
		require.Equal(t, queue.len(), 0)
	})
}
