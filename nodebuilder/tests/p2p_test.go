//go:build p2p || integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
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
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t)

	// create and start BN
	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)
	sw.SetBootstrapper(t, bridge)

	addr := host.InfoFromHost(bridge.Host)

	full := sw.NewBridgeNode()
	light := sw.NewLightNode()

	for _, nd := range []*nodebuilder.Node{full, light} {
		// start node and ensure that BN is correctly set as bootstrapper
		require.NoError(t, nd.Start(ctx))
		// ensure that node is actually connected to BN
		client := getAdminClient(ctx, nd, t)
		connectedenss, err := client.P2P.Connectedness(ctx, addr.ID)
		require.NoError(t, err)
		assert.Equal(t, connectedenss, network.Connected)
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
	cfg := sw.DefaultTestConfig(node.Bridge)
	setTimeInterval(cfg, defaultTimeInterval)
	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)
	err := bridge.Start(ctx)
	require.NoError(t, err)

	// use BN as the bootstrapper
	sw.SetBootstrapper(t, bridge)

	// create FN with BN as bootstrapper
	cfg = sw.DefaultTestConfig(node.Bridge)
	setTimeInterval(cfg, defaultTimeInterval)
	full := sw.NewNodeWithConfig(node.Bridge, cfg)

	// create LN with BN as bootstrapper
	cfg = sw.DefaultTestConfig(node.Light)
	setTimeInterval(cfg, defaultTimeInterval)
	light := sw.NewNodeWithConfig(node.Light, cfg)

	// start FN and LN and ensure they are both connected to BN as a bootstrapper
	nodes := []*nodebuilder.Node{full, light}
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		// ensure that node is actually connected to BN
		client := getAdminClient(ctx, nodes[index], t)
		connectedness, err := client.P2P.Connectedness(ctx, bridge.Host.ID())
		require.NoError(t, err)
		assert.Equal(t, connectedness, network.Connected)
	}

	for {
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
		// LN discovered FN successfully and is now connected
		client := getAdminClient(ctx, light, t)
		connectedness, err := client.P2P.Connectedness(ctx, host.InfoFromHost(full.Host).ID)
		require.NoError(t, err)
		if connectedness == network.Connected {
			break
		}
	}
}

/*
Test-Case: Full node discovery of disconnected full nodes
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create 2 FNs with bridge node as bootstrapper peer and start them
4. Check that the FNs discover each other
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
	fullCfg := sw.DefaultTestConfig(node.Bridge)
	setTimeInterval(fullCfg, defaultTimeInterval)
	bridge := sw.NewNodeWithConfig(node.Bridge, fullCfg)
	err := bridge.Start(ctx)
	require.NoError(t, err)
	sw.SetBootstrapper(t, bridge)

	fullCfg = sw.DefaultTestConfig(node.Bridge)
	setTimeInterval(fullCfg, defaultTimeInterval)

	// create two FNs and start them, ensuring they are connected to BN as
	// bootstrapper
	nodes := make([]*nodebuilder.Node, numFulls)
	for index := 0; index < numFulls; index++ {
		nodes[index] = sw.NewNodeWithConfig(node.Bridge, fullCfg)
		require.NoError(t, nodes[index].Start(ctx))
		client := getAdminClient(ctx, nodes[index], t)
		connectedness, err := client.P2P.Connectedness(ctx, bridge.Host.ID())
		require.NoError(t, err)
		assert.Equal(t, connectedness, network.Connected)
	}

	// ensure FNs are connected to each other
	fullClient1 := getAdminClient(ctx, nodes[0], t)
	fullClient2 := getAdminClient(ctx, nodes[1], t)

	connectedness, err := fullClient1.P2P.Connectedness(ctx, nodes[1].Host.ID())
	require.NoError(t, err)
	assert.Equal(t, connectedness, network.Connected)

	// disconnect the FNs
	sw.Disconnect(t, nodes[0], nodes[1])

	// create and start one more FN with disabled discovery
	disabledDiscoveryFN := sw.NewNodeWithConfig(node.Bridge, fullCfg)
	require.NoError(t, err)
	err = disabledDiscoveryFN.Start(ctx)
	require.NoError(t, err)

	// ensure that the FN with disabled discovery is discovered by both of the
	// running FNs that have discovery enabled
	connectedness, err = fullClient1.P2P.Connectedness(ctx, disabledDiscoveryFN.Host.ID())
	require.NoError(t, err)
	assert.Equal(t, connectedness, network.Connected)

	connectedness, err = fullClient2.P2P.Connectedness(ctx, disabledDiscoveryFN.Host.ID())
	require.NoError(t, err)
	assert.Equal(t, connectedness, network.Connected)
}
