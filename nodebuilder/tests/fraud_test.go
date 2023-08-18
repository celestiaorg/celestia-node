package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

/*
Test-Case: Full Node will propagate a fraud proof to the network, once ByzantineError will be received from sampling.
Pre-Requisites:
- CoreClient is started by swamp.
Steps:
1. Create a Bridge Node(BN) with broken extended header at height 10.
2. Start a BN.
3. Create a Full Node(FN) with a connection to BN as a trusted peer.
4. Start a FN.
5. Subscribe to a fraud proof and wait when it will be received.
6. Check FN is not synced to 15.
Note: 15 is not available because DASer/Syncer will be stopped before reaching this height due to receiving a fraud proof.
Another note: this test disables share exchange to speed up test results.
*/
func TestFraudProofHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	const (
		blocks    = 15
		blockSize = 4
		blockTime = time.Second
	)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, blockSize, blocks)
	set, val := sw.Validators(t)
	fMaker := headertest.NewFraudMaker(t, 10, []types.PrivValidator{val}, set)

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)
	require.NoError(t, edsStore.Start(ctx))
	t.Cleanup(func() {
		_ = edsStore.Stop(ctx)
	})

	cfg := nodebuilder.DefaultConfig(node.Bridge)
	bridge := sw.NewNodeWithConfig(
		node.Bridge,
		cfg,
		core.WithHeaderConstructFn(fMaker.MakeExtendedHeader(16, edsStore)),
		fx.Replace(edsStore),
	)
	err = bridge.Start(ctx)
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Full)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = false
	store := nodebuilder.MockStore(t, cfg)
	full := sw.NewNodeWithStore(node.Full, store)

	err = full.Start(ctx)
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(ctx)
	subscr, err := full.FraudServ.Subscribe(subCtx, byzantine.BadEncoding)
	require.NoError(t, err)
	select {
	case p := <-subscr:
		require.Equal(t, 10, int(p.Height()))
		subCancel()
	case <-ctx.Done():
		subCancel()
		t.Fatal("fraud proof was not received in time")
	}

	// This is an obscure way to check if the Syncer was stopped.
	// If we cannot get a height header within a timeframe it means the syncer was stopped
	// FIXME: Eventually, this should be a check on service registry managing and keeping
	//  lifecycles of each Module.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), blockTime*5)
	_, err = full.HeaderServ.WaitForHeight(syncCtx, 15)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	syncCancel()

	fN := sw.NewNodeWithStore(node.Full, store)
	require.Error(t, fN.Start(ctx))
	proofs, err := fN.FraudServ.Get(ctx, byzantine.BadEncoding)
	require.NoError(t, err)
	require.NotNil(t, proofs)

	sw.StopNode(ctx, bridge)
	sw.StopNode(ctx, full)
	require.NoError(t, <-fillDn)
}
