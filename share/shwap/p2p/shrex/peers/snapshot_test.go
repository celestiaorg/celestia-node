package peers

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestPoolSnapshot(t *testing.T) {
	t.Run("reports status, counts, cooldown and downloading", func(t *testing.T) {
		p := newPool(testPoolParams(time.Minute))
		p.add("peer1", "peer2", "peer3")

		// peer1 is actively downloading and has served a 4096-byte request in 100ms
		p.acquire("peer1")
		p.recordOutcome("peer1", true, 100*time.Millisecond, 4096)

		// peer2 failed and is now on an (adaptive) cooldown
		p.recordOutcome("peer2", false, 0, 0)

		// peer3 is removed and must not appear in the snapshot
		p.remove("peer3")

		snap := p.snapshot()

		require.Equal(t, 1, snap.ActiveCount) // only peer1 active (peer2 cooling, peer3 removed)
		require.Equal(t, 2, snap.TotalCount)  // peer1 + peer2, removed peer excluded
		require.Len(t, snap.Peers, 2)         // removed peer excluded

		byID := map[string]PeerSnapshot{}
		for _, ps := range snap.Peers {
			byID[ps.ID] = ps
		}

		peer1 := byID[peer.ID("peer1").String()]
		require.Equal(t, "active", peer1.Status)
		require.Equal(t, 1, peer1.InFlight)
		require.True(t, peer1.Downloading)
		require.Equal(t, 1, peer1.TotalSuccess)
		require.Equal(t, int64(4096), peer1.TotalBytes)
		// 4096 bytes / 0.1s = ~40960 bytes/sec
		require.InDelta(t, 40960, peer1.ThroughputEWMABytesPerSec, 1)
		require.Nil(t, peer1.CooldownUntil)

		peer2 := byID[peer.ID("peer2").String()]
		require.Equal(t, "cooldown", peer2.Status)
		require.Equal(t, 1, peer2.TotalFailure)
		require.Equal(t, 1, peer2.ConsecFails)
		require.NotNil(t, peer2.CooldownUntil)
		require.True(t, peer2.CooldownUntil.After(time.Now()))
	})

	t.Run("peers sorted by selection score descending", func(t *testing.T) {
		p := newPool(testPoolParams(time.Minute))
		p.add("good", "bad")

		// make "bad" look worse via failures without cooling it (cooldown would drop it
		// from selection); record failures on a peer we keep active by using putBack.
		st := p.stats[peer.ID("bad")]
		st.successEWMA = 0.1
		st.latencyEWMA = 5

		snap := p.snapshot()
		require.Len(t, snap.Peers, 2)
		require.Equal(t, peer.ID("good").String(), snap.Peers[0].ID)
		require.GreaterOrEqual(t, snap.Peers[0].SelectionScore, snap.Peers[1].SelectionScore)
	})
}

func TestManagerSnapshot(t *testing.T) {
	m, err := NewManager(*DefaultParameters(), nil, nil, "full")
	require.NoError(t, err)

	m.nodes.add("node1", "node2")

	snap := m.Snapshot()
	require.Equal(t, "full", snap.Tag)
	require.Equal(t, 2, snap.Nodes.ActiveCount)
	require.Len(t, snap.Nodes.Peers, 2)
	require.Empty(t, snap.DataHashPools)
	require.Zero(t, snap.BlacklistedHashes)
}
