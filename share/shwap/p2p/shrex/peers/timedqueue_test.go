package peers

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestTimedQueue(t *testing.T) {
	const peer1, peer2 = "peer1", "peer2"
	for _, remove := range []peer.ID{"", peer1, peer2, "missing"} {
		t.Run("remove "+string(remove), func(t *testing.T) {
			mock := clock.NewMock()
			fired := make(chan struct{}, 2)
			queue := newTimedQueue(time.Second, func() { fired <- struct{}{} })
			queue.clock = mock
			var popped []peer.ID
			onPop := func(id peer.ID) { popped = append(popped, id) }
			queue.releaseExpired(onPop)
			require.Zero(t, queue.len())

			queue.push(peer1)
			mock.Add(time.Second / 2)
			queue.push(peer2)
			timer := queue.after
			queue.remove(remove)
			if remove == peer1 {
				require.NotSame(t, timer, queue.after)
			} else {
				require.Same(t, timer, queue.after)
			}
			mock.Add(time.Second/2 - 1)
			queue.releaseExpired(onPop)
			require.Empty(t, popped)
			require.Empty(t, fired)

			mock.Add(1)
			if remove == peer1 {
				require.Empty(t, fired)
			} else {
				require.Len(t, fired, 1)
				<-fired
				queue.releaseExpired(onPop)
				require.Equal(t, []peer.ID{peer1}, popped)
			}

			mock.Add(time.Second / 2)
			if remove == peer2 {
				require.Empty(t, fired)
			} else {
				require.Len(t, fired, 1)
				<-fired
				queue.releaseExpired(onPop)
				require.Equal(t, peer.ID(peer2), popped[len(popped)-1])
			}
			require.Zero(t, queue.len())
			require.Nil(t, queue.after)
		})
	}

	t.Run("remove last entry stops timer", func(t *testing.T) {
		mock := clock.NewMock()
		fired := make(chan struct{}, 1)
		queue := newTimedQueue(time.Second, func() { fired <- struct{}{} })
		queue.clock = mock
		queue.push(peer1)
		queue.remove(peer1)
		require.Zero(t, queue.len())
		require.Nil(t, queue.after)
		mock.Add(time.Second)
		require.Empty(t, fired)
	})
}
