package p2p

import (
	"container/heap"
	"context"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
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
	stats := heap.Pop(&pQueue).(*peerStat)
	require.Equal(t, stats.peerID, peer.ID("peerID"))
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

	// we do not need timeout/cancel functionality here
	pQueue := newPeerQueue(context.Background(), peersStat)
	for index := 0; index < pQueue.stats.Len(); index++ {
		stats := heap.Pop(&pQueue.stats).(*peerStat)
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

	// we do not need timeout/cancel functionality here
	pQueue := newPeerQueue(context.Background(), peersStat)

	_ = heap.Pop(&pQueue.stats)
	stat := heap.Pop(&pQueue.stats).(*peerStat)
	require.Equal(t, stat.peerID, peer.ID("peerID2"))
}

func Test_StatsUpdateStats(t *testing.T) {
	// we do not need timeout/cancel functionality here
	pQueue := newPeerQueue(context.Background(), []*peerStat{})
	stat := &peerStat{peerID: "peerID", peerScore: 0}
	heap.Push(&pQueue.stats, stat)
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
		updatedStat := heap.Pop(&pQueue.stats).(*peerStat)
		require.Equal(t, updatedStat.score(), stat.score())
		heap.Push(&pQueue.stats, updatedStat)
	}
}

func Test_StatDecreaseScore(t *testing.T) {
	pStats := &peerStat{
		peerID:    peer.ID("test"),
		peerScore: 100,
	}
	// will decrease score by 20%
	pStats.decreaseScore()
	require.Equal(t, pStats.score(), float32(80.0))
}
