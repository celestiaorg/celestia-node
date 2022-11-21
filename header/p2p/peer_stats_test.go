package p2p

import (
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
	peersStat := peerStats{
		{peerID: "peerID1", peerScore: 1},
		{peerID: "peerID2", peerScore: 2},
		{peerID: "peerID3", peerScore: 4},
		{peerID: "peerID4"}, // score = 0
	}
	wantStat := peerStats{
		{peerID: "peerID3", peerScore: 4},
		{peerID: "peerID2", peerScore: 2},
		{peerID: "peerID1", peerScore: 1},
		{peerID: "peerID4"}, // score = 0
	}
	pQueue := newPeerQueue(peersStat)
	for index := 0; index < pQueue.stats.Len(); index++ {
		stats := pQueue.stats.Pop()
		require.Equal(t, stats, wantStat[index])
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

	_ = pQueue.stats.Pop()
	stat := pQueue.stats.Pop().(*peerStat)
	require.Equal(t, stat.peerID, peer.ID("peerID2"))
}

func Test_StatsUpdateStats(t *testing.T) {
	pQueue := newPeerQueue([]*peerStat{})
	stat := &peerStat{peerID: "peerID", peerScore: 0}
	pQueue.push(stat)
	testCases := []struct {
		inputTime   uint64
		inputBytes  uint64
		resultScore float32
	}{
		// common case, where time and bytes is not equal to 0
		{
			inputTime:   16,
			inputBytes:  4,
			resultScore: 4,
		},
		// in case if bytes is equal to 0,
		// then the request was failed and previous score will be
		// decreased
		{
			inputTime:   10,
			inputBytes:  0,
			resultScore: 2,
		},
		// testing case with time=0, to ensure that dividing by 0 is handled properly
		{
			inputTime:   0,
			inputBytes:  0,
			resultScore: 1,
		},
	}

	for _, tt := range testCases {
		stat.updateStats(tt.inputBytes, tt.inputTime)
		updatedStat := pQueue.stats.Pop().(*peerStat)
		require.Equal(t, updatedStat.score(), stat.score())
		pQueue.push(updatedStat)
	}
}
