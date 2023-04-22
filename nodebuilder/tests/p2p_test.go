package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

/*
Test-Case: Full/Light Nodes connection to Bridge as a Bootstrapper
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create full/light nodes with bridge node as bootstrap peer
4. Start full/light nodes
5. Check that nodes are connected to bridge
*/
func TestBridgeNodeAsBootstrapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)

	// create and start BN
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	addr := host.InfoFromHost(bridge.Host)

	full := sw.NewFullNode(nodebuilder.WithBootstrappers([]peer.AddrInfo{*addr}))
	light := sw.NewLightNode(nodebuilder.WithBootstrappers([]peer.AddrInfo{*addr}))

	for _, nd := range []*nodebuilder.Node{full, light} {
		// start node and ensure that BN is correctly set as bootstrapper
		require.NoError(t, nd.Start(ctx))
		assert.Equal(t, *addr, nd.Bootstrappers[0])
		// ensure that node is actually connected to BN
		assert.True(t, nd.Host.Network().Connectedness(addr.ID) == network.Connected)
	}
}

/*
Test-Case: Connect Full And Light using Bridge node as a bootstrapper
Steps:
 1. Create a Bridge Node(BN)
 2. Start a BN
 3. Create full/light nodes with bridge node as bootstrapped peer
 4. Start full/light nodes
 5. Ensure that nodes are connected to bridge
 6. Wait until light will find full node
 7. Check that full and light nodes are connected to each other
*/
func TestFullDiscoveryViaBootstrapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	const defaultTimeInterval = time.Second * 2

	sw := swamp.NewSwamp(t)

	// create and start a BN
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	setTimeInterval(cfg, defaultTimeInterval)
	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)
	err := bridge.Start(ctx)
	require.NoError(t, err)

	// use BN as the bootstrapper
	bootstrapper := host.InfoFromHost(bridge.Host)

	// create FN with BN as bootstrapper
	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	full := sw.NewNodeWithConfig(
		node.Full,
		cfg,
		nodebuilder.WithBootstrappers([]peer.AddrInfo{*bootstrapper}),
	)

	// create LN with BN as bootstrapper
	cfg = nodebuilder.DefaultConfig(node.Light)
	setTimeInterval(cfg, defaultTimeInterval)
	light := sw.NewNodeWithConfig(
		node.Light,
		cfg,
		nodebuilder.WithBootstrappers([]peer.AddrInfo{*bootstrapper}),
	)

	// start FN and LN and ensure they are both connected to BN as a bootstrapper
	nodes := []*nodebuilder.Node{full, light}
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, *bootstrapper, nodes[index].Bootstrappers[0])
		assert.True(t, nodes[index].Host.Network().Connectedness(bootstrapper.ID) == network.Connected)
	}

	for {
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
		if light.Host.Network().Connectedness(host.InfoFromHost(full.Host).ID) == network.Connected {
			// LN discovered FN successfully and is now connected
			break
		}
	}
}

/*
Test-Case: Full node discovery of disconnected full nodes
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create 2 full nodes with bridge node as bootstrapper peer and start them
4. Check that nodes are connected to each other
5. Disconnect the FNs
6. Create one more node with discovery process disabled (however advertisement is still enabled)
7. Check that the FN with discovery disabled is still found by the other two FNs
*NOTE*: this test will take some time because it relies on several cycles of peer discovery
*/
func TestRestartNodeDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	const (
		defaultTimeInterval = time.Second * 2
		numFulls            = 2
	)

	sw := swamp.NewSwamp(t)

	// create and start a BN as a bootstrapper
	fullCfg := nodebuilder.DefaultConfig(node.Bridge)
	setTimeInterval(fullCfg, defaultTimeInterval)
	bridge := sw.NewNodeWithConfig(node.Bridge, fullCfg)
	err := bridge.Start(ctx)
	require.NoError(t, err)

	bridgeAddr := host.InfoFromHost(bridge.Host)

	fullCfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(fullCfg, defaultTimeInterval)
	nodesConfig := nodebuilder.WithBootstrappers([]peer.AddrInfo{*bridgeAddr})

	// create two FNs and start them, ensuring they are connected to BN as
	// bootstrapper
	nodes := make([]*nodebuilder.Node, numFulls)
	for index := 0; index < numFulls; index++ {
		nodes[index] = sw.NewNodeWithConfig(node.Full, fullCfg, nodesConfig)
		require.NoError(t, nodes[index].Start(ctx))
		assert.True(t, nodes[index].Host.Network().Connectedness(bridgeAddr.ID) == network.Connected)
	}

	// ensure FNs are connected to each other
	require.True(t, nodes[0].Host.Network().Connectedness(nodes[1].Host.ID()) == network.Connected)

	// disconnect the FNs
	sw.Disconnect(t, nodes[0], nodes[1])

	// create and start one more FN with disabled discovery
	fullCfg.Share.Discovery.PeersLimit = 0
	disabledDiscoveryFN := sw.NewNodeWithConfig(node.Full, fullCfg, nodesConfig)
	err = disabledDiscoveryFN.Start(ctx)
	require.NoError(t, err)

	// ensure that the FN with disabled discovery is discovered by both of the
	// running FNs that have discovery enabled
	require.True(t, nodes[0].Host.Network().Connectedness(disabledDiscoveryFN.Host.ID()) == network.Connected)
	require.True(t, nodes[1].Host.Network().Connectedness(disabledDiscoveryFN.Host.ID()) == network.Connected)
}

func setTimeInterval(cfg *nodebuilder.Config, interval time.Duration) {
	cfg.P2P.RoutingTableRefreshPeriod = interval
	cfg.Share.Discovery.AdvertiseInterval = interval
}
