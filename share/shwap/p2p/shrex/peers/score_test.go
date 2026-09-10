package peers

import (
	"strconv"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestPeerStats(t *testing.T) {
	t.Run("unmeasured peer has no score", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		_, measured := s.score("peer1")
		require.False(t, measured)
	})

	t.Run("first transfer sets score", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		s.updateStats("peer1", TransferStats{Bytes: 100 << 20, Duration: time.Second})

		score, measured := s.score("peer1")
		require.True(t, measured)
		require.Equal(t, float64(100<<20), score)
	})

	t.Run("unmeasured peer gets priority once", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		s.updateStats("measured", TransferStats{Bytes: 100 << 20, Duration: time.Second})
		require.Equal(t, peer.ID("unmeasured"), s.selectPeer("measured", "unmeasured"))

		_, measured := s.score("unmeasured")
		require.False(t, measured)
		require.Equal(t, peer.ID("measured"), s.selectPeer("measured", "unmeasured"))
	})

	t.Run("score moves towards observed rate", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		s.updateStats("peer1", TransferStats{Bytes: 1 << 20, Duration: time.Second})
		s.updateStats("peer1", TransferStats{Bytes: 5 << 20, Duration: time.Second})

		score, measured := s.score("peer1")
		require.True(t, measured)
		require.Equal(t, float64(2<<20), score)
	})

	t.Run("failed request decreases score", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		s.updateStats("peer1", TransferStats{Bytes: 100 << 20, Duration: time.Second})
		s.decreaseScore("peer1")

		score, measured := s.score("peer1")
		require.True(t, measured)
		require.Equal(t, float64(100<<20)*peerScoreDecay, score)
	})

	t.Run("failed unmeasured peer gets a reduced default score", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		s.decreaseScore("peer1")

		score, measured := s.score("peer1")
		require.True(t, measured)
		require.Equal(t, float64(defaultPeerScore)*peerScoreDecay, score)
	})

	t.Run("empty stats are ignored", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		s.updateStats("peer1", TransferStats{Bytes: 0, Duration: time.Second})
		s.updateStats("peer1", TransferStats{Bytes: 1 << 20, Duration: 0})

		_, measured := s.score("peer1")
		require.False(t, measured)
	})

	t.Run("peer stats stay bounded", func(t *testing.T) {
		s, err := newPeerStats()
		require.NoError(t, err)

		for i := range peerStatsCacheSize * 3 {
			s.updateStats(peer.ID(strconv.Itoa(i)), TransferStats{Bytes: 1, Duration: time.Second})
		}

		require.Equal(t, peerStatsCacheSize, s.scores.Len())
	})
}
