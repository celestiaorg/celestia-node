package tests

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
)

/*
Test-Case: Sync a Light Node with a Bridge Node
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced to height 30
*/
func TestSyncLightWithBridge(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.DefaultComponents())

	bridge := sw.NewBridgeNode()

	ctx := context.Background()

	state := bridge.CoreClient.IsRunning()
	require.True(t, state)
	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	var ch int64 = 20
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockByHeight(ctx, &ch).BlockID.Hash)

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(sw.Network.Host(bridge.Host.ID())))
	require.NoError(t, err)
	light := sw.NewLightNode(node.WithTrustedPeer(addrs[0].String()))

	err = light.Start(ctx)
	require.NoError(t, err)

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	ch = 30
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockByHeight(ctx, &ch).BlockID.Hash)
}

/*
Test-Case: Light Node continues sync after abrupt stop/start
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 50 blocks
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced to height 30
7. Stop LN
8. Start LN
9. Check LN is synced to height 40
*/
func TestSyncStartStopLightWithBridge(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.DefaultComponents())
	ctx := context.Background()

	sw.WaitTillHeight(ctx, 50)
	bridge := sw.NewBridgeNode()
	state := bridge.CoreClient.IsRunning()
	require.True(t, state)
	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	var ch int64 = 20
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockByHeight(ctx, &ch).BlockID.Hash)

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(sw.Network.Host(bridge.Host.ID())))
	require.NoError(t, err)

	store := node.MockStore(t, node.DefaultConfig(node.Light))
	light := sw.NewLightNodeWithStore(store, node.WithTrustedPeer(addrs[0].String()))
	require.NoError(t, light.Start(ctx))

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	ch = 30
	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockByHeight(ctx, &ch).BlockID.Hash)

	require.NoError(t, light.Stop(ctx))
	require.NoError(t, sw.RemoveNode(light, node.Light))

	light = sw.NewLightNodeWithStore(store, node.WithTrustedPeer(addrs[0].String()))
	require.NoError(t, light.Start(ctx))

	h, err = light.HeaderServ.GetByHeight(ctx, 40)
	require.NoError(t, err)
	ch = 40
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockByHeight(ctx, &ch).BlockID.Hash)
}
