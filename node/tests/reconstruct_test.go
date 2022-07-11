// Test with light nodes spawns more goroutines than in the race detectors budget,
// and thus we're disabling the race detector.
// TODO(@Wondertan): Remove this once we move to go1.19 with unlimited race detector
//go:build !race

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
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
	go sw.FillBlocks(ctx, t, bsize, blocks)

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

	err = errg.Wait()
	require.NoError(t, err)
}

/*
Test-Case: Full Node reconstructs blocks only from Light Nodes
Pre-Reqs:
- First 20 blocks have a block size of 16
- Blocktime is 100 ms
Steps:
1. Create a Bridge Node(BN)
2. Start a BN
3. Create 69 Light Nodes(LNs) with BN as a trusted peer
4. Start 69 LNs
5. Create a Full Node(FN) with 69 LNs as trusted peers
6. Unlink FN connection to BN
7. Start a FN
8. Check that a FN can retrieve shares from 1 to 20 blocks
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	go sw.FillBlocks(ctx, t, bsize, blocks)

	cfg := node.DefaultConfig(node.Bridge)
	cfg.P2P.Bootstrapper = true
	bridge := sw.NewBridgeNode(node.WithConfig(cfg), node.WithRefreshRoutingTablePeriod(time.Second*30))
	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := host.InfoFromHost(bridge.Host)

	full := sw.NewFullNode(
		node.WithBootstrappers([]peer.AddrInfo{*addr}),
		node.WithRefreshRoutingTablePeriod(time.Second*30),
	)
	ch := make(chan struct{}, lnodes-1)
	bundle := &network.NotifyBundle{}
	bundle.ConnectedF = func(_ network.Network, conn network.Conn) {
		if conn.RemotePeer() == full.Host.ID() {
			ch <- struct{}{}
		}
	}
	lights := make([]*node.Node, lnodes)
	for i := 0; i < lnodes; i++ {
		go func(i int) {
			light := sw.NewLightNode(
				node.WithBootstrappers([]peer.AddrInfo{*addr}),
				node.WithRefreshRoutingTablePeriod(time.Second*30),
			)
			err = light.Start(ctx)
			light.Host.Network().Notify(bundle)
			require.NoError(t, err)
			lights[i] = light
		}(i)
	}
	err = full.Start(ctx)
	require.NoError(t, err)
	for i := 0; i < lnodes; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("peer was not found")
		case <-ch:
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

	err = errg.Wait()
	require.NoError(t, err)
}

func getMultiAddr(t *testing.T, h host.Host) string {
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(h))
	require.NoError(t, err)
	return addrs[0].String()
}
