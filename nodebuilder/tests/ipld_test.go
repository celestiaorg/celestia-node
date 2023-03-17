package tests

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
func TestBlockSync_IPLD_SyncLatest(t *testing.T) {
	fullNodesCount := 5

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, 128, 10)

	bridge := sw.NewBridgeNode()

	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.WaitTillHeight(ctx, 10)

	_, err = bridge.HeaderServ.GetByHeight(ctx, 10)
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
			_, err := f.HeaderServ.GetByHeight(ctx, 10)
			if err != nil {
				t.Fatal(err)
			}

			for i := 1; i < 10; i++ {
				h, err := f.HeaderServ.GetByHeight(ctx, uint64(i))
				if err != nil {
					t.Fatal(err)
				}

				err = f.ShareServ.SharesAvailable(ctx, h.DAH)
				fmt.Println("getting shares for height: ", i)
				if err != nil {
					t.Fatal(err)
				}
			}
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
func TestBlockSync_IPLD_SyncLatest_NetworkPartition(t *testing.T) {
	fullNodesCount := 7

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, 64, 30)
	require.NoError(t, <-fillDn)

	logging.SetAllLoggers(logging.LevelError)
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
	for i, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			defer wg.Done()
			_, err := f.HeaderServ.GetByHeight(ctx, 15)
			if err != nil {
				assert.Nil(t, err)
			}

			for j := 1; j <= 15; j++ {
				h, err := f.HeaderServ.GetByHeight(ctx, uint64(j))
				if err != nil {
					assert.Nil(t, err)
				}

				err = f.ShareServ.SharesAvailable(ctx, h.DAH)
				t.Logf("full node #%d getting shares for height: %d", i, j)
				if err != nil {
					assert.Nil(t, err)
				}
			}
		}(full)
	}
	wg.Wait()

	t.Log("All full nodes are synced to height 15")

	// the full node that will remain connected to the bridge after the network partition
	netEntrypointId := rand.Intn(fullNodesCount)

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

	t.Log("All full nodes are disconnected from the bridge node exept for the full node with id: ", netEntrypointId)
	t.Log("Continuing to sync full nodes to height 30")

	wg.Add(fullNodesCount)

	for i, full := range fullNodes {
		go func(f *nodebuilder.Node) {
			defer wg.Done()
			_, err := f.HeaderServ.GetByHeight(ctx, 30)
			if err != nil {
				assert.Nil(t, err)
			}

			for j := 16; j <= 30; j++ {
				h, err := f.HeaderServ.GetByHeight(ctx, uint64(j))
				if err != nil {
					assert.Nil(t, err)
				}

				err = f.ShareServ.SharesAvailable(ctx, h.DAH)
				t.Logf("full node #%d getting shares for height: %d", i, j)
				t.Log()
				if err != nil {
					assert.Nil(t, err)
				}
			}
		}(full)
	}

	wg.Wait()

	t.Log("All full nodes are synced to height 30")
}
