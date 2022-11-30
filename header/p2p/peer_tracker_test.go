package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeerTracker_GC(t *testing.T) {
	h := createMocknet(t, 1)
	p := newPeerTracker(h[0], time.Second*1, time.Millisecond*500, 1)
	pid1 := peer.ID("peer1")
	pid2 := peer.ID("peer2")
	p.connectedPeers[pid1] = &peerStat{peerID: pid1, peerScore: 0.5}
	p.disconnectedPeers[pid2] = &peerStat{peerID: pid2, removedAt: time.Now()}
	assert.True(t, len(p.connectedPeers) > 0)
	assert.True(t, len(p.disconnectedPeers) > 0)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	t.Cleanup(cancel)
	p.gc(ctx)

	require.True(t, len(p.connectedPeers) == 0)
	require.True(t, len(p.disconnectedPeers) == 0)
}
