//go:build reconstruction || integration

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/eds"
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
	t.Skip()
	const (
		blocks = 20
		bsize  = 16
		btime  = time.Millisecond * 300
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)
	bridgeClient := getAdminClient(ctx, bridge, t)

	// TODO: This is required to avoid flakes coming from unfinished retry
	// mechanism for the same peer in go-header
	_, err = bridgeClient.Header.WaitForHeight(ctx, uint64(blocks))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Share.UseShareExchange = false
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, getMultiAddr(t, bridge.Host))
	full := sw.NewNodeWithConfig(node.Full, cfg)
	err = full.Start(ctx)
	require.NoError(t, err)
	fullClient := getAdminClient(ctx, full, t)

	errg, bctx := errgroup.WithContext(ctx)
	for i := 1; i <= blocks+1; i++ {
		i := i
		errg.Go(func() error {
			h, err := fullClient.Header.WaitForHeight(bctx, uint64(i))
			if err != nil {
				return err
			}

			return fullClient.Share.SharesAvailable(bctx, h)
		})
	}
	require.NoError(t, <-fillDn)
	require.NoError(t, errg.Wait())
}

/*
Test-Case: Full Node reconstructs blocks from each other, after unsuccessfully syncing the complete
block from LN subnetworks. Analog to TestShareAvailable_DisconnectedFullNodes.
*/
func TestFullReconstructFromFulls(t *testing.T) {
	t.Skip()
	if testing.Short() {
		t.Skip()
	}

	light.DefaultSampleAmount = 10 // s
	const (
		blocks = 10
		btime  = time.Millisecond * 300
		bsize  = 8  // k
		lnodes = 12 // c - total number of nodes on two subnetworks
	)

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	const defaultTimeInterval = time.Second * 5
	bridge := sw.NewBridgeNode()

	sw.SetBootstrapper(t, bridge)
	require.NoError(t, bridge.Start(ctx))
	bridgeClient := getAdminClient(ctx, bridge, t)

	// TODO: This is required to avoid flakes coming from unfinished retry
	// mechanism for the same peer in go-header
	_, err := bridgeClient.Header.WaitForHeight(ctx, uint64(blocks))
	require.NoError(t, err)

	lights1 := make([]*nodebuilder.Node, lnodes/2)
	lights2 := make([]*nodebuilder.Node, lnodes/2)
	subs := make([]event.Subscription, lnodes)
	errg, errCtx := errgroup.WithContext(ctx)
	for i := 0; i < lnodes/2; i++ {
		i := i
		errg.Go(func() error {
			lnConfig := nodebuilder.DefaultConfig(node.Light)
			setTimeInterval(lnConfig, defaultTimeInterval)
			light := sw.NewNodeWithConfig(node.Light, lnConfig)
			sub, err := light.Host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
			if err != nil {
				return err
			}
			subs[i] = sub
			lights1[i] = light
			return light.Start(errCtx)
		})
		errg.Go(func() error {
			lnConfig := nodebuilder.DefaultConfig(node.Light)
			setTimeInterval(lnConfig, defaultTimeInterval)
			light := sw.NewNodeWithConfig(node.Light, lnConfig)
			sub, err := light.Host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
			if err != nil {
				return err
			}
			subs[(lnodes/2)+i] = sub
			lights2[i] = light
			return light.Start(errCtx)
		})
	}

	require.NoError(t, errg.Wait())

	for i := 0; i < lnodes; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("peer was not found")
		case <-subs[i].Out():
			require.NoError(t, subs[i].Close())
			continue
		}
	}

	// Remove bootstrappers to prevent FNs from connecting to bridge
	sw.Bootstrappers = []ma.Multiaddr{}
	// Use light nodes from respective subnetworks as bootstrappers to prevent connection to bridge
	lnBootstrapper1, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(lights1[0].Host))
	require.NoError(t, err)
	lnBootstrapper2, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(lights2[0].Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Share.UseShareExchange = false
	cfg.Share.Discovery.PeersLimit = 0
	cfg.Header.TrustedPeers = []string{lnBootstrapper1[0].String()}
	full1 := sw.NewNodeWithConfig(node.Full, cfg)
	cfg.Header.TrustedPeers = []string{lnBootstrapper2[0].String()}
	full2 := sw.NewNodeWithConfig(node.Full, cfg)
	require.NoError(t, full1.Start(ctx))
	require.NoError(t, full2.Start(ctx))

	fullClient1 := getAdminClient(ctx, full1, t)
	fullClient2 := getAdminClient(ctx, full2, t)

	// Form topology
	for i := 0; i < lnodes/2; i++ {
		// Separate light nodes into two subnetworks
		for j := 0; j < lnodes/2; j++ {
			sw.Disconnect(t, lights1[i], lights2[j])
			if i != j {
				sw.Connect(t, lights1[i], lights1[j])
				sw.Connect(t, lights2[i], lights2[j])
			}
		}

		sw.Connect(t, full1, lights1[i])
		sw.Disconnect(t, full1, lights2[i])

		sw.Connect(t, full2, lights2[i])
		sw.Disconnect(t, full2, lights1[i])
	}

	// Ensure the fulls are not connected to the bridge
	sw.Disconnect(t, full1, full2)
	sw.Disconnect(t, full1, bridge)
	sw.Disconnect(t, full2, bridge)

	h, err := fullClient1.Header.WaitForHeight(ctx, uint64(10+blocks-1))
	require.NoError(t, err)

	// Ensure that the full nodes cannot reconstruct before being connected to each other
	ctxErr, cancelErr := context.WithTimeout(ctx, time.Second*30)
	errg, errCtx = errgroup.WithContext(ctxErr)
	errg.Go(func() error {
		return fullClient1.Share.SharesAvailable(errCtx, h)
	})
	errg.Go(func() error {
		return fullClient1.Share.SharesAvailable(errCtx, h)
	})
	require.Error(t, errg.Wait())
	cancelErr()

	// Reconnect FNs
	sw.Connect(t, full1, full2)

	errg, bctx := errgroup.WithContext(ctx)
	for i := 10; i < blocks+11; i++ {
		h, err := fullClient1.Header.WaitForHeight(bctx, uint64(i))
		require.NoError(t, err)
		errg.Go(func() error {
			return fullClient1.Share.SharesAvailable(bctx, h)
		})
		errg.Go(func() error {
			return fullClient2.Share.SharesAvailable(bctx, h)
		})
	}

	require.NoError(t, <-fillDn)
	require.NoError(t, errg.Wait())
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
	t.Skip()
	if testing.Short() {
		t.Skip()
	}

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
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], bsize, blocks)

	const defaultTimeInterval = time.Second * 5
	cfg := nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)

	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	os.Setenv(p2p.EnvKeyCelestiaBootstrapper, "true")
	cfg.Header.TrustedPeers = []string{
		"/ip4/1.2.3.4/tcp/12345/p2p/12D3KooWNaJ1y1Yio3fFJEXCZyd1Cat3jmrPdgkYCrHfKD3Ce21p",
	}
	bootstrapper := sw.NewNodeWithConfig(node.Full, cfg)
	require.NoError(t, bootstrapper.Start(ctx))
	bootstrapperAddr := host.InfoFromHost(bootstrapper.Host)

	require.NoError(t, bridge.Start(ctx))
	bridgeClient := getAdminClient(ctx, bridge, t)

	// TODO: This is required to avoid flakes coming from unfinished retry
	// mechanism for the same peer in go-header
	_, err = bridgeClient.Header.WaitForHeight(ctx, uint64(blocks))
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg, defaultTimeInterval)
	cfg.Share.UseShareExchange = false
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrsBridge[0].String())
	nodesConfig := nodebuilder.WithBootstrappers([]peer.AddrInfo{*bootstrapperAddr})
	full := sw.NewNodeWithConfig(node.Full, cfg, nodesConfig)
	os.Setenv(p2p.EnvKeyCelestiaBootstrapper, "false")

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
	fullClient := getAdminClient(ctx, full, t)

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
			h, err := fullClient.Header.WaitForHeight(bctx, uint64(i))
			if err != nil {
				return err
			}

			return fullClient.Share.SharesAvailable(bctx, h)
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
