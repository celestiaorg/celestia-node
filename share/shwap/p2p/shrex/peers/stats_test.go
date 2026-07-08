package peers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPeerStats(t *testing.T) {
	t.Run("optimistic init scores high", func(t *testing.T) {
		params := DefaultParameters()
		now := time.Now()
		st := newPeerStats(params, now)
		// unproven peer should look attractive so it gets tried
		require.Equal(t, 1.0, st.successEWMA)
		require.Greater(t, st.quality(now), 0.9)
	})

	t.Run("failures decay quality, successes restore it", func(t *testing.T) {
		params := DefaultParameters()
		now := time.Now()
		st := newPeerStats(params, now)

		for i := 0; i < 10; i++ {
			st.recordFailure(params, now)
		}
		failed := st.quality(now)
		require.Less(t, failed, 0.1)

		for i := 0; i < 10; i++ {
			st.recordSuccess(50*time.Millisecond, params, now)
		}
		require.Greater(t, st.quality(now), failed)
	})

	t.Run("faster peer scores higher", func(t *testing.T) {
		params := DefaultParameters()
		now := time.Now()
		fast := newPeerStats(params, now)
		slow := newPeerStats(params, now)

		for i := 0; i < 20; i++ {
			fast.recordSuccess(50*time.Millisecond, params, now)
			slow.recordSuccess(5*time.Second, params, now)
		}
		require.Greater(t, fast.quality(now), slow.quality(now))
	})

	t.Run("in-flight lowers selection score", func(t *testing.T) {
		params := DefaultParameters()
		now := time.Now()
		idle := newPeerStats(params, now)
		busy := newPeerStats(params, now)
		busy.inFlight = 3

		require.Greater(t, idle.selectionScore(now, params), busy.selectionScore(now, params))
	})

	t.Run("at-cap peer is heavily penalized", func(t *testing.T) {
		params := DefaultParameters()
		now := time.Now()
		below := newPeerStats(params, now)
		below.inFlight = params.InflightCap - 1
		atCap := newPeerStats(params, now)
		atCap.inFlight = params.InflightCap

		// the at-cap peer should score far below one just under the cap
		require.Less(t, atCap.selectionScore(now, params), below.selectionScore(now, params)/10)
	})

	t.Run("adaptive cooldown escalates and caps", func(t *testing.T) {
		params := DefaultParameters()
		params.PeerCooldown = time.Second
		params.CooldownFactor = 2
		params.MaxCooldown = 10 * time.Second
		st := newPeerStats(params, time.Now())

		st.consecFails = 1
		require.Equal(t, time.Second, st.adaptiveCooldown(params))
		st.consecFails = 2
		require.Equal(t, 2*time.Second, st.adaptiveCooldown(params))
		st.consecFails = 3
		require.Equal(t, 4*time.Second, st.adaptiveCooldown(params))
		// escalation is capped at MaxCooldown
		st.consecFails = 20
		require.Equal(t, 10*time.Second, st.adaptiveCooldown(params))
	})

	t.Run("idle peer gains freshness bonus", func(t *testing.T) {
		params := DefaultParameters()
		start := time.Now()
		st := newPeerStats(params, start)
		st.recordSuccess(50*time.Millisecond, params, start)

		fresh := st.quality(start)
		later := st.quality(start.Add(freshnessWindow))
		require.Greater(t, later, fresh)
	})
}
