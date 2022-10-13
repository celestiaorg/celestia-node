package p2p

import (
	"sort"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
)

func Test_PeerStatsPush(t *testing.T) {
	pQueue := newPeerStats()
	pQueue.Push(&peerStat{peerID: "peerID"})
	require.True(t, pQueue.Len() == 1)
}

func Test_PeerStatsPop(t *testing.T) {
	pQueue := newPeerStats()
	pQueue.Push(&peerStat{peerID: "peerID"})
	stats := pQueue.Pop()
	pStat, ok := stats.(*peerStat)
	require.True(t, ok)
	require.Equal(t, pStat.peerID, peer.ID("peerID"))
}

func Test_PeerQueuePopBestPeer(t *testing.T) {
	peersStat := []*peerStat{
		{peerID: "peerID1", peerScore: 1},
		{peerID: "peerID2", peerScore: 2},
		{peerID: "peerID3", peerScore: 4},
		{peerID: "peerID4"}, // score = 0
	}
	pQueue := newPeerQueue(peersStat)

	sort.Slice(peersStat, func(i, j int) bool {
		return peersStat[i].peerScore > peersStat[j].peerScore
	})

	for index := 0; index < pQueue.len(); index++ {
		stats := pQueue.pop()
		require.Equal(t, stats, peersStat[index])
	}
}

func Test_PeerQueueRemovePeer(t *testing.T) {
	peersStat := []*peerStat{
		{peerID: "peerID1", peerScore: 1},
		{peerID: "peerID2", peerScore: 2},
		{peerID: "peerID3", peerScore: 4},
		{peerID: "peerID4"}, // score = 0
	}
	pQueue := newPeerQueue(peersStat)

	_ = pQueue.pop()
	stat := pQueue.pop()
	require.Equal(t, stat.peerID, peer.ID("peerID2"))
}

func Test_StatsUpdateStats(t *testing.T) {
	pQueue := newPeerQueue([]*peerStat{})
	stat := &peerStat{peerID: "peerID3", peerScore: 4}
	pQueue.push(stat)

	stat.peerScore = 10
	updatedStat := pQueue.pop()
	require.Equal(t, stat.peerScore, updatedStat.peerScore)

	updatedStat.peerScore = 20
	require.Equal(t, stat.peerScore, updatedStat.peerScore)
}
