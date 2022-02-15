package tests

import (
	"context"
	"testing"
	"time"

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
3. Check BN is synced
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced with BN
*/
func TestSyncLightWithBridge(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.DefaultInfraComponents())

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
