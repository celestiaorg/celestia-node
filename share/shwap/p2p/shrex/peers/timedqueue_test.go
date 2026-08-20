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

		popCh := make(chan peer.ID, 1)
		queue := newTimedQueue(ttl, func(id peer.ID, _ uint64) {
			popCh <- id
		})
		mock := clock.NewMock()
		queue.clock = mock
		queue.releaseExpired()
		require.Zero(t, queue.len())

		// push first item | global time : 0
		queue.push(peers[0])
		require.Equal(t, queue.len(), 1)

		// push second item with ttl/2 gap | global time : ttl/2
		mock.Add(ttl / 2)
		queue.push(peers[1])
		require.Equal(t, queue.len(), 2)

		// advance clock 1 nano sec before first item should expire | global time : ttl - 1
		mock.Add(ttl/2 - 1)
		// check that releaseExpired doesn't remove items
		queue.releaseExpired()
		require.Equal(t, queue.len(), 2)
		// first item should be released after its own timeout | global time : ttl
		mock.Add(1)

		select {
		case id := <-popCh:
			require.Equal(t, peers[0], id)
		case <-time.After(ttl):
			t.Fatal("first item is not released")
		}
		require.Equal(t, queue.len(), 1)

		// first item should be released after ttl/2 gap timeout | global time : 3/2*ttl
		mock.Add(ttl / 2)
		select {
		case id := <-popCh:
			require.Equal(t, peers[1], id)
		case <-time.After(ttl):
			t.Fatal("second item is not released")
		}
		require.Equal(t, queue.len(), 0)
	})

	t.Run("callback does not hold queue lock", func(t *testing.T) {
		ttl := time.Second
		mock := clock.NewMock()
		callbackDone := make(chan peer.ID, 1)

		var queue *timedQueue
		queue = newTimedQueue(ttl, func(id peer.ID, _ uint64) {
			queue.push("peer2")
			callbackDone <- id
		})
		queue.clock = mock
		queue.push("peer1")

		advanceDone := make(chan struct{})
		go func() {
			mock.Add(ttl)
			close(advanceDone)
		}()

		select {
		case id := <-callbackDone:
			require.Equal(t, peer.ID("peer1"), id)
		case <-time.After(ttl):
			t.Fatal("callback blocked while accessing queue")
		}

		select {
		case <-advanceDone:
		case <-time.After(ttl):
			t.Fatal("clock advance did not finish")
		}
		require.Equal(t, 1, queue.len())
	})
}
