package peers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestScoreboard tests that peers are seeded optimistically and that observed transfers move
// their score towards the observed rate.
func TestScoreboard(t *testing.T) {
	t.Run("unmeasured peer gets optimistic seed", func(t *testing.T) {
		s, err := newScoreboard()
		require.NoError(t, err)

		require.Equal(t, s.seed, s.score("peer1"))
	})

	t.Run("score moves towards observed rate", func(t *testing.T) {
		s, err := newScoreboard()
		require.NoError(t, err)

		rate := 100 << 20
		for range 100 {
			s.observe("peer1", Sample{Bytes: int64(rate), Duration: time.Second})
		}
		require.InEpsilon(t, float64(rate), s.score("peer1"), 0.01)
	})

	t.Run("slow peer scores below seed", func(t *testing.T) {
		s, err := newScoreboard()
		require.NoError(t, err)

		s.observe("peer1", Sample{Bytes: 1 << 10, Duration: time.Second})
		require.Less(t, s.score("peer1"), s.seed)
	})

	t.Run("empty samples are ignored", func(t *testing.T) {
		s, err := newScoreboard()
		require.NoError(t, err)

		s.observe("peer1", Sample{Bytes: 0, Duration: time.Second})
		s.observe("peer1", Sample{Bytes: 1 << 20, Duration: 0})
		require.Equal(t, s.seed, s.score("peer1"))
	})
}
