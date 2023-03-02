package tests

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"

	logging "github.com/ipfs/go-log/v2"
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
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

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
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

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
Test-Case: Sync a group of Full Node with a Bridge Node with Network Partitions(includes DASing of non-empty blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height 1
4. Create {fullNodescount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs are synced to height 15
7. Disconnect all FNs (except a randomly chosen one) from BN
8. Check FNs are synced to height 30
*/
func TestSyncFullsWithBridge_Network_Partitioned_Latest_IPLD(t *testing.T) {
	fullNodesCount := 16

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, 1)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 1)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 1))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = false // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	waiter := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, 15)
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 15))
	}

	// the full node that will remain connected to the bridge after the network partition
	rand.Seed(16)
	netEntrypointId := rand.Int()

	// Let full nodes black list the bridge node (except for the full node with netEntrypointId)
	bridgeInfo := host.InfoFromHost(bridge.Host)
	for id, full := range fullNodes {
		if id != netEntrypointId {
			full.Host.Network().ClosePeer(bridgeInfo.ID)
			full.ConnGater.BlockPeer(bridgeInfo.ID)
			if !full.ConnGater.InterceptPeerDial(bridgeInfo.ID) {
				t.Log("Blocked (bridge) maddr=", bridgeInfo.Addrs[0], "Bye bye.")
			}
		}
	}

	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, 30)
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
}

/*
Test-Case: Sync a group of Full Nodes with a Bridge Node (includes DASing of non-empty blocks) after {targetHeight} has been reached.

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height {targetHeight}
4. Create {fullNodesCount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs are synced up to {targetHeight}
6. Check FNs are synced up to latest height???
*/
func TestSyncFullsWithBridge_Network_Stable_Historical_IPLD(t *testing.T) {
	fullNodesCount := 16
	targetHeight := 15
	topHeight := 30

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, int64(targetHeight))

	h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 1))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = false // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	waitForFulls := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
			waitForFulls <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waitForFulls
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(targetHeight)))
	}

	sw.WaitTillHeight(ctx, int64(topHeight))

	h, err = bridge.HeaderServ.GetByHeight(ctx, uint64(topHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))

	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(topHeight))
			waitForFulls <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waitForFulls
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
}

/*
Test-Case: Sync a group of Full Node with a Bridge Node(includes DASing of non-empty blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 1
4. Create 32 Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start the FNs
6. Check FNs are synced to height 15
7. Disconnect all FNs (except one) from the bridge node
8. Check FNs are synced to height 30
*/
func TestSyncFullsWithBridge_Network_Partitioned_Historical_IPLD(t *testing.T) {
	fullNodesCount := 16
	targetHeight := 15
	topHeight := 30

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, int64(targetHeight))

	h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(targetHeight)))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = false // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	waiter := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(targetHeight)))
	}

	// the full node that will remain connected to the bridge after the network partition
	rand.Seed(16)
	netEntrypointId := rand.Int()

	// Let full nodes black list the bridge node (except for the full node with netEntrypointId)
	bridgeInfo := host.InfoFromHost(bridge.Host)
	for id, full := range fullNodes {
		if id != netEntrypointId {
			full.Host.Network().ClosePeer(bridgeInfo.ID)
			full.ConnGater.BlockPeer(bridgeInfo.ID)
			if !full.ConnGater.InterceptPeerDial(bridgeInfo.ID) {
				t.Log("Blocked (bridge) maddr=", bridgeInfo.Addrs[0], "Bye bye.")
			}
		}
	}

	sw.WaitTillHeight(ctx, int64(topHeight))

	h, err = bridge.HeaderServ.GetByHeight(ctx, uint64(topHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))

	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(topHeight))
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
}

/*
Test-Case: Sync a group of Full Nodes with a Bridge Node (includes DASing of non-empty blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height 1
4. Create {fullNodesCount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs are synced to height 30
*/
func TestSyncFullsWithBridge_Network_Stable_Latest_Shrex(t *testing.T) {
	fullNodesCount := 16

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, 1)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 1)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 1))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = true // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	sw.WaitTillHeight(ctx, 30)
	h, err = bridge.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	waitForFulls := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, 30)
			waitForFulls <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waitForFulls
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
}

/*
Test-Case: Sync a group of Full Node with a Bridge Node with Network Partitions(includes DASing of non-empty blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height 1
4. Create {fullNodescount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs are synced to height 15
7. Disconnect all FNs (except a randomly chosen one) from BN
8. Check FNs are synced to height 30
*/
func TestSyncFullsWithBridge_Network_Partitioned_Latest_Shrex(t *testing.T) {
	fullNodesCount := 16

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, 1)

	h, err := bridge.HeaderServ.GetByHeight(ctx, 1)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 1))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = true // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	waiter := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, 15)
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 15))
	}

	// the full node that will remain connected to the bridge after the network partition
	rand.Seed(16)
	netEntrypointId := rand.Int()

	// Let full nodes black list the bridge node (except for the full node with netEntrypointId)
	bridgeInfo := host.InfoFromHost(bridge.Host)
	for id, full := range fullNodes {
		if id != netEntrypointId {
			full.Host.Network().ClosePeer(bridgeInfo.ID)
			full.ConnGater.BlockPeer(bridgeInfo.ID)
			if !full.ConnGater.InterceptPeerDial(bridgeInfo.ID) {
				t.Log("Blocked (bridge) maddr=", bridgeInfo.Addrs[0], "Bye bye.")
			}
		}
	}

	sw.WaitTillHeight(ctx, 30)

	h, err = bridge.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))

	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, 30)
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 30))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
}

/*
Test-Case: Sync a group of Full Nodes with a Bridge Node (includes DASing of non-empty blocks) after {targetHeight} has been reached.

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height {targetHeight}
4. Create {fullNodesCount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs are synced up to {targetHeight}
6. Check FNs are synced up to latest height???
*/
func TestSyncFullsWithBridge_Network_Stable_Historical_Shrex(t *testing.T) {
	fullNodesCount := 16
	targetHeight := 15
	topHeight := 30

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, int64(targetHeight))

	h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, 1))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = true // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	waitForFulls := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
			waitForFulls <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waitForFulls
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(targetHeight)))
	}

	sw.WaitTillHeight(ctx, int64(topHeight))

	h, err = bridge.HeaderServ.GetByHeight(ctx, uint64(topHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))

	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(topHeight))
			waitForFulls <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waitForFulls
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
}

/*
Test-Case: Sync a group of Full Node with a Bridge Node(includes DASing of non-empty blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Check BN is synced to height 1
4. Create 32 Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start the FNs
6. Check FNs are synced to height 15
7. Disconnect all FNs (except one) from the bridge node
8. Check FNs are synced to height 30
*/
func TestSyncFullsWithBridge_Network_Partitioned_Historical_Shrex(t *testing.T) {
	fullNodesCount := 16
	targetHeight := 15
	topHeight := 30

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

	logging.SetAllLoggers(logging.LevelError)
	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, int64(targetHeight))

	h, err := bridge.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(targetHeight)))

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = true // use IPLD only

	fullNodes := make([]*nodebuilder.Node, 0)
	for i := 1; i <= fullNodesCount; i++ {
		logging.SetAllLoggers(logging.LevelError)
		fullNodes = append(fullNodes, sw.NewNodeWithConfig(node.Full, cfg))
	}

	for _, full := range fullNodes {
		require.NoError(t, full.Start(ctx))
	}

	type syncNotif struct {
		h   *header.ExtendedHeader
		err error
	}

	waiter := make(chan syncNotif)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(targetHeight))
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(targetHeight)))
	}

	// the full node that will remain connected to the bridge after the network partition
	rand.Seed(16)
	netEntrypointId := rand.Int()

	// Let full nodes black list the bridge node (except for the full node with netEntrypointId)
	bridgeInfo := host.InfoFromHost(bridge.Host)
	for id, full := range fullNodes {
		if id != netEntrypointId {
			full.Host.Network().ClosePeer(bridgeInfo.ID)
			full.ConnGater.BlockPeer(bridgeInfo.ID)
			if !full.ConnGater.InterceptPeerDial(bridgeInfo.ID) {
				t.Log("Blocked (bridge) maddr=", bridgeInfo.Addrs[0], "Bye bye.")
			}
		}
	}

	sw.WaitTillHeight(ctx, int64(topHeight))

	h, err = bridge.HeaderServ.GetByHeight(ctx, uint64(topHeight))
	require.NoError(t, err)
	assert.EqualValues(t, h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))

	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			h, err := f.HeaderServ.GetByHeight(ctx, uint64(topHeight))
			waiter <- syncNotif{h, err}
		}(full)
	}

	for i := 1; i < fullNodesCount; i++ {
		res := <-waiter
		require.NoError(t, res.err)
		assert.EqualValues(t, res.h.Commit.BlockID.Hash, sw.GetCoreBlockHashByHeight(ctx, int64(topHeight)))
	}

	require.NoError(t, <-fillDn)

	for _, full := range fullNodes {
		require.NoError(t, full.Stop(ctx))
	}
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
