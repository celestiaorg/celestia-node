package tests

import (
	"context"
	"fmt"
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
3. Check BN is synced
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced with BN
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
	fmt.Println(h.Commit.Hash().String())

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(sw.Network.Host(bridge.Host.ID())))
	require.NoError(t, err)
	light := sw.NewLightNode(node.WithTrustedPeer(addrs[0].String()))

	err = light.Start(ctx)
	require.NoError(t, err)

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	var ch int64 = 30
	b, err := sw.CoreClient.Block(ctx, &ch)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, b.BlockID.Hash)
}
