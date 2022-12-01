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
	p.connectedPeers[pid1] = &peerStat{peerID: pid1, peerScore: 0.5}
	p.disconnectedPeers[pid2] = &peerStat{peerID: pid2, removedAt: time.Now()}
	assert.True(t, len(p.connectedPeers) > 0)
	assert.True(t, len(p.disconnectedPeers) > 0)

	go p.track()
	go p.gc()
	time.Sleep(time.Second * 1)
	p.stop()
	require.True(t, len(p.connectedPeers) == 0)
	require.True(t, len(p.disconnectedPeers) == 0)
}
