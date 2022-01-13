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
Test-Case: Sync a Light Client with a Bridge Node
Steps:
1. Create a bridge node(BN)
2. Start a BN
3. Check BN is synced
4. Create a Light Client(LC) with a trusted peer
5. Start a LC with a defined connection to the BN
6. Check LC is synced with BN
*/
func TestSyncLightWithBridge(t *testing.T) {
	sw := NewSwamp(t)
	bridge := sw.NewBridgeNode()

	ctx := context.Background()

	state := bridge.CoreClient.IsRunning()
	require.True(t, state)
	sw.WaitTillHeight(30)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	assert.False(t, bridge.HeaderServ.IsSyncing())

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(sw.Network.Host(bridge.Host.ID())))
	require.NoError(t, err)
	light := sw.NewLightClient(node.WithTrustedPeer(addrs[0].String()))

	require.NoError(t, sw.Network.LinkAll())
	err = light.Start(ctx)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	assert.False(t, light.HeaderServ.IsSyncing())
}
