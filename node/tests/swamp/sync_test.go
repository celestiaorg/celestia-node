package swamp

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node"
)

/*
Test-Case: Sync a Light Node with a Bridge Node
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced with BN
*/
func TestSyncLightWithBridge(t *testing.T) {
	tendermint := NewTendermintCoreNode(100, 100*time.Millisecond)
	sw := NewSwamp(t, tendermint)

	bridge := sw.NewBridgeNode()

	ctx := context.Background()

	state := bridge.CoreClient.IsRunning()
	require.True(t, state)
	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	assert.False(t, bridge.HeaderServ.IsSyncing())

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(sw.Network.Host(bridge.Host.ID())))
	require.NoError(t, err)
	light := sw.NewLightNode(node.WithTrustedPeer(addrs[0].String()))

	require.NoError(t, sw.Network.LinkAll())
	err = light.Start(ctx)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	assert.False(t, light.HeaderServ.IsSyncing())
}

func TestStartStopSyncLightWithBridge(t *testing.T) {
	tendermint := NewTendermintCoreNode(50, 100*time.Millisecond)
	sw := NewSwamp(t, tendermint)

	bridge := sw.NewBridgeNode()

	ctx := context.Background()

	state := bridge.CoreClient.IsRunning()
	require.True(t, state)
	sw.WaitTillHeight(ctx, 10)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	assert.False(t, bridge.HeaderServ.IsSyncing())

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(sw.Network.Host(bridge.Host.ID())))
	require.NoError(t, err)

	cfg := node.DefaultConfig(node.Light)
	store := node.MockStore(t, cfg)

	light := sw.NewLightNodeWithStore(store, node.WithTrustedPeer(addrs[0].String()))

	require.NoError(t, sw.Network.LinkAll())

	err = light.Start(ctx)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	assert.False(t, light.HeaderServ.IsSyncing())

	err = light.Stop(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, 30)

	light = sw.NewLightNodeWithStore(store, node.WithTrustedPeer(addrs[0].String()))
	err = light.Start(ctx)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	assert.False(t, light.HeaderServ.IsSyncing())

}

// /ip6/100:0:d71f:7a5a:6731:51ee:dc5b:e02b/tcp/4242/p2p/12D3KooWJxi9FMHxRzKZpEHUYEd2HqedFX4fcW6FmJ7HzySeqxRv
// /ip6/100:0:d71f:7a5a:6731:51ee:dc5b:e02b/tcp/4242/p2p/12D3KooWJxi9FMHxRzKZpEHUYEd2HqedFX4fcW6FmJ7HzySeqxRv
