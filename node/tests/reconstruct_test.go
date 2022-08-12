// Test with light nodes spawns more goroutines than in the race detectors budget,
// and thus we're disabling the race detector.
// TODO(@Wondertan): Remove this once we move to go1.19 with unlimited race detector
//go:build !race

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
	"github.com/celestiaorg/celestia-node/service/share"
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
		btime  = time.Millisecond * 100
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	errCh := make(chan error)
	go func() {
		errCh <- sw.FillBlocks(ctx, bsize, blocks)
	}()

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	full := sw.NewFullNode(node.WithTrustedPeers(getMultiAddr(t, bridge.Host)))
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
	require.NoError(t, <-errCh)
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
3. Create a Full Node(FN) that will act as a bootstraper
4. Create 69 Light Nodes(LNs) with BN as a trusted peer and a bootstaper
5. Start 69 LNs
6. Create a Full Node(FN) with a bootstraper
7. Unlink FN connection to BN
8. Start a FN
9. Check that a FN can retrieve shares from 1 to 20 blocks
*/
func TestFullReconstructFromLights(t *testing.T) {
	ipld.RetrieveQuadrantTimeout = time.Millisecond * 100
	share.DefaultSampleAmount = 20
	const (
		blocks = 20
		btime  = time.Millisecond * 100
		bsize  = 16
		lnodes = 69
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	errCh := make(chan error)
	go func() {
		errCh <- sw.FillBlocks(ctx, bsize, blocks)
	}()

	const defaultTimeInterval = time.Second * 5
	var defaultOptions = []node.Option{
		node.WithRefreshRoutingTablePeriod(defaultTimeInterval),
		node.WithDiscoveryInterval(defaultTimeInterval),
		node.WithAdvertiseInterval(defaultTimeInterval),
	}

	cfg := node.DefaultConfig(node.Full)
	cfg.P2P.Bootstrapper = true
	bridge := sw.NewBridgeNode()
	addrsBridge, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	bootstrapConfig := append([]node.Option{node.WithConfig(cfg)}, defaultOptions...)
	bootstapFN := sw.NewFullNode(bootstrapConfig...)
	require.NoError(t, bootstapFN.Start(ctx))
	require.NoError(t, bridge.Start(ctx))
	addrBootstrapNode := host.InfoFromHost(bootstapFN.Host)

	nodesConfig := append(
		[]node.Option{
			node.WithTrustedPeers(addrsBridge[0].String()),
			node.WithBootstrappers([]peer.AddrInfo{*addrBootstrapNode})},
		defaultOptions...,
	)
	full := sw.NewFullNode(nodesConfig...)
	lights := make([]*node.Node, lnodes)
	subs := make([]event.Subscription, lnodes)
	errg, errCtx := errgroup.WithContext(ctx)
	for i := 0; i < lnodes; i++ {
		i := i
		errg.Go(func() error {
			lnConfig := append(
				[]node.Option{
					node.WithTrustedPeers(addrsBridge[0].String())},
				nodesConfig...,
			)
			light := sw.NewLightNode(lnConfig...)
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
	require.NoError(t, <-errCh)
	require.NoError(t, errg.Wait())
}

func getMultiAddr(t *testing.T, h host.Host) string {
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(h))
	require.NoError(t, err)
	return addrs[0].String()
}
