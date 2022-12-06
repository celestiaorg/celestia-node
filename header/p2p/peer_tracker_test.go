package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeerTracker_GC(t *testing.T) {
	h := createMocknet(t, 1)
	p := newPeerTracker(h[0], time.Millisecond*200, time.Millisecond*1, 1, 5)
	pid1 := peer.ID("peer1")
	pid2 := peer.ID("peer2")
	pid3 := peer.ID("peer3")
	pid4 := peer.ID("peer4")
	p.connectedPeers[pid1] = &peerStat{peerID: pid1, peerScore: 0.5}
	p.connectedPeers[pid2] = &peerStat{peerID: pid2, peerScore: 10}
	p.disconnectedPeers[pid3] = &peerStat{peerID: pid3, pruneDeadline: time.Now()}
	p.disconnectedPeers[pid4] = &peerStat{peerID: pid4, pruneDeadline: time.Now().Add(time.Minute * 10)}
	assert.True(t, len(p.connectedPeers) > 0)
	assert.True(t, len(p.disconnectedPeers) > 0)

	go p.track()
	go p.gc()
	time.Sleep(time.Second * 1)
	p.stop()
	require.Nil(t, p.connectedPeers[pid1])
	require.Nil(t, p.disconnectedPeers[pid3])
}
