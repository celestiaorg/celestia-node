package tests

import (
	"context"
	logging "github.com/ipfs/go-log/v2"
	"math/rand"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

/*
Test-Case: Sync a group of Full Nodes with a Bridge Node (includes DASing of non-empty 128-square size blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height 30
4. Create {fullNodesCount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs sync *and* DAS to height 30
*/
func TestSyncFullsWithBridge_Network_Stable_Latest_IPLD(t *testing.T) {
	fullNodesCount := 1

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, 128, 30)

	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, 30)

	_, err = bridge.HeaderServ.GetByHeight(ctx, 30)
	require.NoError(t, err)

	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = false // use IPLD only

	fullNodes := make([]*nodebuilder.Node, fullNodesCount)
	for i := range fullNodes {
		fullNodes[i] = sw.NewNodeWithConfig(node.Full, cfg)
		err = fullNodes[i].Start(ctx)
		require.NoError(t, err)
	}

	wg := sync.WaitGroup{}
	wg.Add(fullNodesCount)
	for _, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			defer wg.Done()
			// first wait for full node to get to height 30
			_, err := f.HeaderServ.GetByHeight(ctx, 30)
			require.NoError(t, err)
			// and ensure it DASes up to that height
			err = f.DASer.WaitCatchUp(ctx)
			require.NoError(t, err)
			stats, err := f.DASer.SamplingStats(ctx)
			require.NoError(t, err)
			assert.Len(t, stats.Failed, 0)
			assert.GreaterOrEqual(t, stats.CatchupHead, uint64(30))
		}(full)
	}
	wg.Wait()

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
