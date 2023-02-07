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

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

/*
Test-Case: Full/Light Nodes connection to Bridge as a bootstrapper
Steps:
1. Create and start a Bridge Node(BN)
2. Create and start Full/Light Nodes(FN/LN) with BN as bootstrapper
3. Check that FN/LN are connected to BN
*/
func TestBootstrapping(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := []peer.AddrInfo{*host.InfoFromHost(bridge.Host)}

	full := sw.NewFullNode(nodebuilder.WithBootstrappers(addr))
	light := sw.NewLightNode(nodebuilder.WithBootstrappers(addr))
	nodes := []*nodebuilder.Node{full, light}
	for index := range nodes {
		require.NoError(t, nodes[index].Start(ctx))
		assert.Equal(t, addr, nodes[index].Bootstrappers)
		assert.True(t, nodes[index].Host.Network().Connectedness(bridge.Host.ID()) == network.Connected)
	}
}

/*
Test-Case: Add peer to blacklist
Steps:
1. Create and start a Full Node(FN)
2. Create and start a Light Node(LN)
3. LN blocks FN
4. Check that LN intercepts and prevents dials from FN
*/
func TestAddPeerToBlackList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))

	full := sw.NewFullNode()
	err := full.Start(ctx)
	require.NoError(t, err)

	light := sw.NewLightNode()
	require.NoError(t, light.Start(ctx))
	require.NoError(t, light.ConnGater.BlockPeer(full.Host.ID()))

	require.False(t, light.ConnGater.InterceptPeerDial(full.Host.ID()))
}

/*
Test-Case: Light discovers Full over bootstrapper
Steps:
 1. Create and start a Bridge Node(BN) bootstrapper
 2. Create and start a Full Node(FN)
 3. Create and start a Light Node(LN)
 4. Wait until LN discovers FN
*/
func TestDiscoveryOverBootstrapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))

	cfg := nodebuilder.DefaultConfig(node.Bridge)
	cfg.P2P.Bootstrapper = true
	setTimeInterval(cfg)
	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)
	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := []peer.AddrInfo{*host.InfoFromHost(bridge.Host)}

	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg)
	full := sw.NewNodeWithConfig(node.Full, cfg, nodebuilder.WithBootstrappers(addr))
	err = full.Start(ctx)
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Light)
	setTimeInterval(cfg)
	light := sw.NewNodeWithConfig(node.Light, cfg, nodebuilder.WithBootstrappers(addr))
	err = light.Start(ctx)
	require.NoError(t, err)

	sub, err := light.Host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)
	defer sub.Close()

	select {
	case e := <-sub.Out():
		if e.(event.EvtPeerConnectednessChanged).Peer != full.Host.ID() {
			t.Fatal("wrong peer event")
		}
		assert.True(t, light.Host.Network().Connectedness(full.Host.ID()) == network.Connected)
	case <-ctx.Done():
		t.Fatal("peer was not connected within a timeout")
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
	const fullNodes = 2

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))

	cfg := nodebuilder.DefaultConfig(node.Bridge)
	cfg.P2P.Bootstrapper = true
	setTimeInterval(cfg)
	cfg.Share.PeersLimit = fullNodes
	bridge := sw.NewNodeWithConfig(node.Bridge, cfg)
	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := []peer.AddrInfo{*host.InfoFromHost(bridge.Host)}

	nodes := make([]*nodebuilder.Node, fullNodes)
	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg)
	cfg.Share.PeersLimit = fullNodes
	for index := 0; index < fullNodes; index++ {
		nodes[index] = sw.NewNodeWithConfig(node.Full, cfg, nodebuilder.WithBootstrappers(addr))
	}

	identitySub, err := nodes[0].Host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	require.NoError(t, err)
	defer identitySub.Close()

	for index := 0; index < fullNodes; index++ {
		require.NoError(t, nodes[index].Start(ctx))
		assert.True(t, nodes[index].Host.Network().Connectedness(bridge.Host.ID()) == network.Connected)
	}

	// wait until full nodes connect each other
	e := <-identitySub.Out()
	connStatus := e.(event.EvtPeerIdentificationCompleted)
	id := connStatus.Peer
	if id != nodes[1].Host.ID() {
		t.Fatal("unexpected peer connected")
	}
	require.True(t, nodes[0].Host.Network().Connectedness(id) == network.Connected)

	// create one more node with disabled discovery
	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg)
	cfg.Share.PeersLimit = 0
	node := sw.NewNodeWithConfig(node.Full, cfg, nodebuilder.WithBootstrappers(addr))
	connectSub, err := nodes[0].Host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)
	defer connectSub.Close()
	sw.Disconnect(nodes[0].Host.ID(), nodes[1].Host.ID())
	err = node.Start(ctx)
	require.NoError(t, err)

	for {
		select {
		case <-ctx.Done():
			require.True(t, nodes[0].Host.Network().Connectedness(node.Host.ID()) == network.Connected)
			return
		case conn := <-connectSub.Out():
			status := conn.(event.EvtPeerConnectednessChanged)
			if status.Peer != node.Host.ID() {
				continue
			}
			require.True(t, status.Connectedness == network.Connected)
			return
		}
	}
}

func setTimeInterval(cfg *nodebuilder.Config) {
	interval := time.Second * 5
	cfg.P2P.RoutingTableRefreshPeriod = interval
	cfg.Share.DiscoveryInterval = interval
	cfg.Share.AdvertiseInterval = interval
}
