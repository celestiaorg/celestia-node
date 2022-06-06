package tests

import (
	"context"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
)

/*
Test-Case: Sync a Light Node with a Bridge Node
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create full/light nodes with bridge node as bootsrapped peer
4. Start full/light nodes
5. Check that nodes are connected to bridge
*/
func TestUseBridgeNodeAsBootstraper(t *testing.T) {
	sw := swamp.NewSwamp(t)

	bridge := sw.NewBridgeNode()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	addr := host.InfoFromHost(bridge.Host)

	full := sw.NewFullNode(node.WithBootstrappers([]peer.AddrInfo{*addr}))
	light := sw.NewLightNode(node.WithBootstrappers([]peer.AddrInfo{*addr}))
	nodes := []*node.Node{full, light}
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, *addr, nodes[index].Bootstrappers[0])
		assert.True(t, nodes[index].Host.Network().Connectedness(addr.ID) == network.Connected)
	}
}

/*
Test-Case: Sync a Light Node with a Bridge Node
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create full/light nodes with bridge node as bootsrapped peer
4. Start full/light nodes
5. Ensure that nodes are connected to bridge
6. Check that full and light nodes are connected to each other
*/
func TestBootstrapNodesFromBridgeNode(t *testing.T) {
	sw := swamp.NewSwamp(t)
	cfg := node.DefaultConfig(node.Bridge)
	cfg.P2P.Bootstrapper = true
	bridge := sw.NewBridgeNode(node.WithConfig(cfg), node.WithRefreshRoutingTablePeriod(time.Second*30))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := host.InfoFromHost(bridge.Host)

	full := sw.NewFullNode(
		node.WithBootstrappers([]peer.AddrInfo{*addr}),
		node.WithRefreshRoutingTablePeriod(time.Second*30),
	)
	light := sw.NewLightNode(
		node.WithBootstrappers([]peer.AddrInfo{*addr}),
		node.WithRefreshRoutingTablePeriod(time.Second*30),
	)
	nodes := []*node.Node{full, light}
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, *addr, nodes[index].Bootstrappers[0])
		assert.True(t, nodes[index].Host.Network().Connectedness(addr.ID) == network.Connected)
	}
	sub0, err := light.Host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)
	defer sub0.Close()
LOOP:
	for {
		select {
		case <-ctx.Done():
			t.Fatal("peer was not found")
		case p := <-sub0.Out():
			ev := p.(event.EvtPeerConnectednessChanged)
			if ev.Peer == full.Host.ID() && ev.Connectedness == network.Connected {
				break LOOP
			}
		}
	}
	addrFull := host.InfoFromHost(full.Host)
	assert.True(t, light.Host.Network().Connectedness(addrFull.ID) == network.Connected)
}
