package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

// Common consts for tests producing filled blocks
const (
	blocks = 20
	bsize  = 16
	btime  = time.Millisecond * 300
)

/*
Test-Case: Sync a Light Node with a Bridge Node(includes DASing of non-empty blocks)
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Light Node(LN) with a trusted peer
5. Start a LN with a defined connection to the BN
6. Check LN is synced to height 30
*/
func TestSyncLightWithBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := core.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()

	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	light := sw.NewNodeWithConfig(node.Light, cfg)

	err = light.Start(ctx)
	require.NoError(t, err)

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	err = light.ShareServ.SharesAvailable(ctx, h.DAH)
	assert.NoError(t, err)

	err = light.DASer.WaitCatchUp(ctx)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	require.NoError(t, <-fillDn)
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
	sw := swamp.NewSwamp(t)

	bridge := sw.NewBridgeNode()

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw.WaitTillHeight(ctx, 50)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	light := sw.NewNodeWithConfig(node.Light, cfg)
	require.NoError(t, light.Start(ctx))

	h, err = light.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	require.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	require.NoError(t, light.Stop(ctx))
	require.NoError(t, sw.RemoveNode(light, node.Light))

	cfg = nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	light = sw.NewNodeWithConfig(node.Light, cfg)
	require.NoError(t, light.Start(ctx))

	h, err = light.HeaderServ.GetByHeight(ctx, 40)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 40))
}

/*
Test-Case: Sync a Full Node with a Bridge Node(includes DASing of non-empty blocks)
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Full Node(FN) with a connection to BN as a trusted peer
5. Start a FN
6. Check FN is synced to height 30
*/
func TestSyncFullWithBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := core.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()

	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	full := sw.NewNodeWithConfig(node.Full, cfg)
	require.NoError(t, full.Start(ctx))

	h, err = full.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	err = full.ShareServ.SharesAvailable(ctx, h.DAH)
	assert.NoError(t, err)

	err = full.DASer.WaitCatchUp(ctx)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	require.NoError(t, <-fillDn)
}

/*
Test-Case: Sync a Light Node from a Full Node
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 20 blocks
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Full Node(FN) with a connection to BN as a trusted peer
5. Start a FN
6. Check FN is synced to height 30
7. Create a Light Node(LN) with a connection to FN as a trusted peer
8. Start LN
9. Check LN is synced to height 50
*/
func TestSyncLightWithFull(t *testing.T) {
	sw := swamp.NewSwamp(t)

	bridge := sw.NewBridgeNode()

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	full := sw.NewNodeWithConfig(node.Full, cfg)
	require.NoError(t, full.Start(ctx))

	h, err = full.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	addrs, err = peer.AddrInfoToP2pAddrs(host.InfoFromHost(full.Host))
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	light := sw.NewNodeWithConfig(node.Light, cfg)

	err = sw.Network.UnlinkPeers(bridge.Host.ID(), light.Host.ID())
	require.NoError(t, err)

	err = light.Start(ctx)
	require.NoError(t, err)

	h, err = light.HeaderServ.GetByHeight(ctx, 50)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 50))
}

/*
Test-Case: Sync a Light Node with multiple trusted peers
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 20 blocks
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 20
4. Create a Full Node(FN) with a connection to BN as a trusted peer
5. Start a FN
6. Check FN is synced to height 30
7. Create a Light Node(LN) with a connection to BN, FN as trusted peers
8. Start LN
9. Check LN is synced to height 50
*/
func TestSyncLightWithTrustedPeers(t *testing.T) {
	sw := swamp.NewSwamp(t)

	bridge := sw.NewBridgeNode()

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw.WaitTillHeight(ctx, 20)

	err := bridge.Start(ctx)
	require.NoError(t, err)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 20)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 20))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	full := sw.NewNodeWithConfig(node.Full, cfg)
	require.NoError(t, full.Start(ctx))

	h, err = full.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	addrs, err = peer.AddrInfoToP2pAddrs(host.InfoFromHost(full.Host))
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	light := sw.NewNodeWithConfig(node.Light, cfg)

	err = light.Start(ctx)
	require.NoError(t, err)

	h, err = light.HeaderServ.GetByHeight(ctx, 50)
	require.NoError(t, err)

	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 50))
}
