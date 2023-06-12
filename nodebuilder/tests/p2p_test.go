package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
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

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	addr := host.InfoFromHost(bridge.Host)

	full := sw.NewFullNode(nodebuilder.WithBootstrappers([]peer.AddrInfo{*addr}))
	light := sw.NewLightNode(nodebuilder.WithBootstrappers([]peer.AddrInfo{*addr}))

	nodes := []*nodebuilder.Node{full, light}
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, *addr, nodes[index].Bootstrappers[0])
		client := getAdminClient(ctx, nodes[index], t)
		connectedness, err := client.P2P.Connectedness(ctx, addr.ID)
		require.NoError(t, err)
		assert.Equal(t, network.Connected, connectedness)
	}
}

/*
Test-Case: Add peer to blacklist
Steps:
1. Create a Full Node(BN)
2. Start a FN
3. Create a Light Node(LN)
5. Start a LN
6. Explicitly block FN id by LN
7. Check FN is allowed to dial with LN
8. Check LN is not allowed to dial with FN
*/
func TestAddPeerToBlackList(t *testing.T) {
	sw := swamp.NewSwamp(t)
	full := sw.NewFullNode()
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	require.NoError(t, full.Start(ctx))

	addr := host.InfoFromHost(full.Host)
	light := sw.NewLightNode()
	require.NoError(t, light.Start(ctx))
	lightClient := getAdminClient(ctx, light, t)
	require.NoError(t, lightClient.P2P.BlockPeer(ctx, addr.ID))

	require.True(t, full.ConnGater.InterceptPeerDial(host.InfoFromHost(light.Host).ID))
	require.False(t, light.ConnGater.InterceptPeerDial(addr.ID))
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
 8. Stop FN and ensure that it's not connected to LN
*/
func TestBootstrapNodesFromBridgeNode(t *testing.T) {
	sw := swamp.NewSwamp(t)
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	const defaultTimeInterval = time.Second * 10
	setTimeInterval(cfg, defaultTimeInterval)

	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	bridgeAddr := host.InfoFromHost(bridge.Host)

	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	full := sw.NewNodeWithConfig(
		node.Full,
		cfg,
		nodebuilder.WithBootstrappers([]peer.AddrInfo{*bridgeAddr}),
	)

	cfg = nodebuilder.DefaultConfig(node.Light)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.P2P.PeerExchange = true
	light := sw.NewNodeWithConfig(
		node.Light,
		cfg,
		nodebuilder.WithBootstrappers([]peer.AddrInfo{*bridgeAddr}),
	)
	nodes := []*nodebuilder.Node{full, light}
	clients := make([]*client.Client, len(nodes))
	ch := make(chan struct{})
	sub, err := light.Host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)
	defer sub.Close()
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, *bridgeAddr, nodes[index].Bootstrappers[0])
		client := getAdminClient(ctx, nodes[index], t)
		clients[index] = client
		connectedness, err := client.P2P.Connectedness(ctx, bridgeAddr.ID)
		require.NoError(t, err)
		assert.Equal(t, network.Connected, connectedness)
	}
	addrFull := host.InfoFromHost(full.Host)
	go func() {
		for e := range sub.Out() {
			connStatus := e.(event.EvtPeerConnectednessChanged)
			if connStatus.Peer == full.Host.ID() && connStatus.Connectedness == network.NotConnected {
				ch <- struct{}{}
			}
		}
	}()

	// ensure that the light node is connected to the full node
	lightClient := clients[1]
	connectedness, err := lightClient.P2P.Connectedness(ctx, addrFull.ID)
	assert.NoError(t, err)
	assert.Equal(t, network.Connected, connectedness)

	sw.Disconnect(t, light, full)
	require.NoError(t, full.Stop(ctx))
	select {
	case <-ctx.Done():
		t.Fatal("peer was not disconnected")
	case <-ch:
		connectedness, err := lightClient.P2P.Connectedness(ctx, addrFull.ID)
		assert.NoError(t, err)
		assert.Equal(t, network.NotConnected, connectedness)
	}
}

/*
Test-Case: Restart full node discovery after one node is disconnected
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create 2 full nodes with bridge node as bootstrapper peer and start them
4. Check that nodes are connected to each other
5. Create one more node with disabled discovery
6. Disconnect FNs from each other
7. Check that the last FN is connected to one of the nodes
*NOTE*: this test will take some time because it relies on several cycles of peer discovery
*/
func TestRestartNodeDiscovery(t *testing.T) {
	sw := swamp.NewSwamp(t)
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	const defaultTimeInterval = time.Second * 2
	const fullNodes = 2

	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Share.Discovery.PeersLimit = fullNodes
	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	bridgeAddr := host.InfoFromHost(bridge.Host)

	nodes := make([]*nodebuilder.Node, fullNodes)
	clients := make([]*client.Client, fullNodes)
	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Share.Discovery.PeersLimit = fullNodes
	nodesConfig := nodebuilder.WithBootstrappers([]peer.AddrInfo{*bridgeAddr})
	for index := 0; index < fullNodes; index++ {
		nodes[index] = sw.NewNodeWithConfig(node.Full, cfg, nodesConfig)
		require.NoError(t, nodes[index].Start(ctx))
		clients[index] = getAdminClient(ctx, nodes[index], t)
	}

	// ensure full nodes are connected to the bridge node
	for index := 0; index < fullNodes; index++ {
		connectedness, err := clients[index].P2P.Connectedness(ctx, bridgeAddr.ID)
		require.NoError(t, err)
		require.Equal(t, network.Connected, connectedness)
	}

	// ensure full nodes are connected to each other
	connectedness, err := clients[0].P2P.Connectedness(ctx, nodes[1].Host.ID())
	require.NoError(t, err)
	require.Equal(t, network.Connected, connectedness)

	// create one more node with disabled discovery
	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Share.Discovery.PeersLimit = 0
	node := sw.NewNodeWithConfig(node.Full, cfg, nodesConfig)
	connectSub, err := nodes[0].Host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)
	defer connectSub.Close()
	sw.Disconnect(t, nodes[0], nodes[1])
	require.NoError(t, node.Start(ctx))

	// ensure that the last node is connected to one of the nodes
	connectedness, err = clients[0].P2P.Connectedness(ctx, node.Host.ID())
	require.NoError(t, err)
	require.Equal(t, network.Connected, connectedness)
}

func setTimeInterval(cfg *nodebuilder.Config, interval time.Duration) {
	cfg.P2P.RoutingTableRefreshPeriod = interval
	cfg.Share.Discovery.AdvertiseInterval = interval
}
