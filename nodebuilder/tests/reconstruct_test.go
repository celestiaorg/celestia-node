// Test with light nodes spawns more goroutines than in the race detectors budget,
// and thus we're disabling the race detector.
// TODO(@Wondertan): Remove this once we move to go1.19 with unlimited race detector
//go:build !race

package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/eds"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
)

/*
Test-Case: Full Node reconstructs blocks from a Bridge node
Pre-Reqs:
- First 20 blocks have a block size of 16
- Blocktime is 100 ms
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create a Full Node(FN) with BN as a trusted peer
4. Start a FN
5. Check that a FN can retrieve shares from 1 to 20 blocks
*/
func TestFullReconstructFromBridge(t *testing.T) {
	const (
		blocks = 20
		bsize  = 16
		btime  = time.Millisecond * 300
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, getMultiAddr(t, bridge.Host))
	full := sw.NewNodeWithConfig(node.Full, cfg)
	err = full.Start(ctx)
	require.NoError(t, err)

	errg, bctx := errgroup.WithContext(ctx)
	for i := 1; i <= blocks+1; i++ {
		i := i
		errg.Go(func() error {
			h, err := full.HeaderServ.GetByHeight(bctx, uint64(i))
			if err != nil {
				return err
			}

			return full.ShareServ.SharesAvailable(bctx, h.DAH)
		})
	}
	require.NoError(t, <-fillDn)
	require.NoError(t, errg.Wait())
}

/*
Test-Case: Sync a group of Full Nodes with a Bridge Node (includes DASing of non-empty blocks)

Steps:
1. Create a Bridge Node(BN)
2. Start BN
3. Check BN is synced to height 30
4. Create {fullNodesCount} Full Nodes(FNs) with a connection to BN as a trusted peer
5. Start FNs
6. Check FNs sync *and* DAS to height 30
*/
func TestSyncFullsWithBridge_Network_Stable_Latest_IPLD(t *testing.T) {
	fullNodesCount := 5

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, 30)

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
			assert.GreaterOrEqual(t, stats.CatchupHead, uint64(30))
		}(full)
	}
	wg.Wait()

	require.NoError(t, <-fillDn)
}

/*
Test-Case: Full Node reconstructs blocks only from Light Nodes
Pre-Reqs:
- First 20 blocks have a block size of 16
- Blocktime is 100 ms
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create a Full Node(FN) that will act as a bootstrapper
4. Create 69 Light Nodes(LNs) with BN as a trusted peer and a bootstrapper
5. Start 69 LNs
6. Create a Full Node(FN) with a bootstrapper
7. Unlink FN connection to BN
8. Start a FN
9. Check that the FN can retrieve shares from 1 to 20 blocks
*/
func TestFullReconstructFromLights(t *testing.T) {
	eds.RetrieveQuadrantTimeout = time.Millisecond * 100
	light.DefaultSampleAmount = 20
	const (
		blocks = 20
		btime  = time.Millisecond * 300
		bsize  = 16
		lnodes = 69
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)

	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)

	const defaultTimeInterval = time.Second * 5
	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.P2P.Bootstrapper = true
	setTimeInterval(cfg, defaultTimeInterval)

	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	bootstrapper := sw.NewNodeWithConfig(node.Full, cfg)
	require.NoError(t, bootstrapper.Start(ctx))
	require.NoError(t, bridge.Start(ctx))
	bootstrapperAddr := host.InfoFromHost(bootstrapper.Host)

	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrsBridge[0].String())
	nodesConfig := nodebuilder.WithBootstrappers([]peer.AddrInfo{*bootstrapperAddr})
	full := sw.NewNodeWithConfig(node.Full, cfg, nodesConfig)

	lights := make([]*nodebuilder.Node, lnodes)
	subs := make([]event.Subscription, lnodes)
	errg, errCtx := errgroup.WithContext(ctx)
	for i := 0; i < lnodes; i++ {
		i := i
		errg.Go(func() error {
			lnConfig := nodebuilder.DefaultConfig(node.Light)
			setTimeInterval(lnConfig, defaultTimeInterval)
			lnConfig.Header.TrustedPeers = append(lnConfig.Header.TrustedPeers, addrsBridge[0].String())
			light := sw.NewNodeWithConfig(node.Light, lnConfig, nodesConfig)
			sub, err := light.Host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
			if err != nil {
				return err
			}
			subs[i] = sub
			lights[i] = light
			return light.Start(errCtx)
		})
	}
	require.NoError(t, errg.Wait())
	require.NoError(t, full.Start(ctx))
	for i := 0; i < lnodes; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("peer was not found")
		case <-subs[i].Out():
			require.NoError(t, subs[i].Close())
			continue
		}
	}
	errg, bctx := errgroup.WithContext(ctx)
	for i := 1; i <= blocks+1; i++ {
		i := i
		errg.Go(func() error {
			h, err := full.HeaderServ.GetByHeight(bctx, uint64(i))
			if err != nil {
				return err
			}

			return full.ShareServ.SharesAvailable(bctx, h.DAH)
		})
	}
	require.NoError(t, <-fillDn)
	require.NoError(t, errg.Wait())
}

func getMultiAddr(t *testing.T, h host.Host) string {
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(h))
	require.NoError(t, err)
	return addrs[0].String()
}
