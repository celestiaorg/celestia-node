package tests

import (
	"context"
	"testing"
	"time"

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
Steps:
1. Create and start a Bridge Node(BN)
2. Create and start a Full Node(FN)
3. Connect the FN to the BN
4. Check that the FN can retrieve shares from 1 to 20 blocks
*/
func TestFullReconstructFromBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))
	fillDn := sw.FillBlocks(ctx, blockSize, blocksAmount)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	full := sw.NewFullNode()
	err = full.Start(ctx)
	require.NoError(t, err)
	sw.Connect(bridge.Host.ID(), full.Host.ID())

	errg, bctx := errgroup.WithContext(ctx)
	for i := 1; i <= blocksAmount+1; i++ {
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
Test-Case: Full Node reconstructs blocks only from Light Nodes with discovery
Steps:
1. Create and start a Light Node as a bootstrapper(so that LNs below are forced to discover BN for data)
2. Create and start a Bridge Node(BN)
3. Create and start a Full Node(FN)
4. Unlink FN from BN(prevents any data syncing between them)
5. Create and start 69 Light Nodes(LNs)
6. Check that the FN can retrieve shares from 1 to 20 blocks
*/
func TestFullReconstructFromLightsWithDiscovery(t *testing.T) {
	eds.RetrieveQuadrantTimeout = time.Millisecond * 100
	light.DefaultSampleAmount = 20
	const lnodes = 69

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))
	fillDn := sw.FillBlocks(ctx, blockSize, blocksAmount)

	cfg := nodebuilder.DefaultConfig(node.Light)
	cfg.P2P.Bootstrapper = true
	setTimeInterval(cfg)
	bootstrapper := sw.NewNodeWithConfig(node.Light, cfg)
	err := bootstrapper.Start(ctx)
	require.NoError(t, err)
	bootstrapperAddr := []peer.AddrInfo{*host.InfoFromHost(bootstrapper.Host)}

	bridge := sw.NewBridgeNode(nodebuilder.WithBootstrappers(bootstrapperAddr))
	err = bridge.Start(ctx)
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Full)
	setTimeInterval(cfg)
	full := sw.NewNodeWithConfig(node.Full, cfg, nodebuilder.WithBootstrappers(bootstrapperAddr))
	sw.Disconnect(full.Host.ID(), bridge.Host.ID())

	errg, errCtx := errgroup.WithContext(ctx)
	for i := 0; i < lnodes; i++ {
		errg.Go(func() error {
			lnConfig := nodebuilder.DefaultConfig(node.Light)
			setTimeInterval(lnConfig)
			light := sw.NewNodeWithConfig(node.Light, lnConfig, nodebuilder.WithBootstrappers(bootstrapperAddr))
			return light.Start(errCtx)
		})
	}

	err = errg.Wait()
	require.NoError(t, err)
	err = full.Start(ctx)
	require.NoError(t, err)

	errg, bctx := errgroup.WithContext(ctx)
	for i := 1; i <= blocksAmount+1; i++ {
		i := i
		errg.Go(func() error {
			h, err := full.HeaderServ.GetByHeight(bctx, uint64(i))
			if err != nil {
				return err
			}

			return full.ShareServ.SharesAvailable(bctx, h.DAH)
		})
	}

	err = <-fillDn
	require.NoError(t, err)
	err = errg.Wait()
	require.NoError(t, err)
}
