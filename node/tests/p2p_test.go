package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
)

/*
Test-Case: Full/Light Nodes connection to Bridge as a Bootstapper
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
Test-Case: Connect Full And Light using Bridge node as a bootstrapper
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create full/light nodes with bridge node as bootsrapped peer
4. Start full/light nodes
5. Ensure that nodes are connected to bridge
6. Wait until light will find full node
7. Check that full and light nodes are connected to each other
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
	ch := make(chan struct{})
	bundle := &network.NotifyBundle{}
	bundle.ConnectedF = func(_ network.Network, conn network.Conn) {
		if conn.RemotePeer() == full.Host.ID() {
			ch <- struct{}{}
		}
	}
	light.Host.Network().Notify(bundle)

	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, *addr, nodes[index].Bootstrappers[0])
		assert.True(t, nodes[index].Host.Network().Connectedness(addr.ID) == network.Connected)
	}

	select {
	case <-ctx.Done():
		t.Fatal("peer was not found")
	case <-ch:
		break
	}

	addrFull := host.InfoFromHost(full.Host)
	assert.True(t, light.Host.Network().Connectedness(addrFull.ID) == network.Connected)
}
