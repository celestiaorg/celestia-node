package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
)

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	require.NoError(t, full.Start(ctx))

	addr := host.InfoFromHost(full.Host)
	light := sw.NewLightNode()
	require.NoError(t, light.Start(ctx))
	require.NoError(t, light.ConnGater.BlockPeer(addr.ID))

	require.True(t, full.ConnGater.InterceptPeerDial(host.InfoFromHost(light.Host).ID))
	require.False(t, light.ConnGater.InterceptPeerDial(addr.ID))
}
