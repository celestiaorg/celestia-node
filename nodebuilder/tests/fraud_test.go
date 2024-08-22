//go:build fraud || integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
	headerfraud "github.com/celestiaorg/celestia-node/header/headertest/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/store"
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
Note: 15 is not available because DASer/Syncer will be stopped
before reaching this height due to receiving a fraud proof.
Another note: this test disables share exchange to speed up test results.
7. Spawn a Light Node(LN) in order to sync a BEFP.
8. Ensure that the BEFP was received.
9. Try to start a Full Node(FN) that contains a BEFP in its store.
*/
func TestFraudProofHandling(t *testing.T) {
	t.Skip()
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	const (
		blocks    = 15
		blockSize = 4
		blockTime = time.Second
	)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(blockTime))
	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts[0], blockSize, blocks)
	set, val := sw.Validators(t)
	fMaker := headerfraud.NewFraudMaker(t, 10, []types.PrivValidator{val}, set)

	storeCfg := store.DefaultParameters()
	edsStore, err := store.NewStore(storeCfg, t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = edsStore.Stop(ctx)
	})

	cfg := nodebuilder.DefaultConfig(node.Bridge)
	// 1.
	bridge := sw.NewNodeWithConfig(
		node.Bridge,
		cfg,
		core.WithHeaderConstructFn(fMaker.MakeExtendedHeader(16, edsStore)),
		fx.Replace(edsStore),
	)
	// 2.
	err = bridge.Start(ctx)
	require.NoError(t, err)

	// 3.
	cfg = nodebuilder.DefaultConfig(node.Full)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	cfg.Share.UseShareExchange = false
	store := nodebuilder.MockStore(t, cfg)
	full := sw.MustNewNodeWithStore(node.Full, store)

	// 4.
	err = full.Start(ctx)
	require.NoError(t, err)

	fullClient := getAdminClient(ctx, full, t)

	// 5.
	subCtx, subCancel := context.WithCancel(ctx)
	subscr, err := fullClient.Fraud.Subscribe(subCtx, byzantine.BadEncoding)
	require.NoError(t, err)
	select {
	case p := <-subscr:
		require.Equal(t, 10, int(p.Height()))
		t.Log("Caught the proof....")
		subCancel()
	case <-ctx.Done():
		subCancel()
		t.Fatal("full node did not receive a fraud proof in time")
	}

	getCtx, getCancel := context.WithTimeout(ctx, time.Second)
	proofs, err := fullClient.Fraud.Get(getCtx, byzantine.BadEncoding)
	getCancel()

	require.NoError(t, err)
	require.Len(t, proofs, 1)
	require.True(t, proofs[0].Type() == byzantine.BadEncoding)
	// This is an obscure way to check if the Syncer was stopped.
	// If we cannot get a height header within a timeframe it means the syncer was stopped
	// FIXME: Eventually, this should be a check on service registry managing and keeping
	//  lifecycles of each Module.
	// 6.
	// random height after befp.height
	height := uint64(15)
	// initial timeout is set to 5 sec, as we are targeting the height=15,
	// blockTime=1 sec, expected befp.height=10
	timeOut := blockTime * 5
	// during befp validation the node can still receive headers and it mostly depends on
	// the operating system or hardware(e.g. on macOS tests is working 100% time with a single
	// height=15, and on the Linux VM sometimes the last height is 17-18). So, lets give a chance for
	// our befp validator to check the fraud proof and stop the syncer.
	for height < 20 {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), timeOut)
		_, err = full.HeaderServ.WaitForHeight(syncCtx, height)
		syncCancel()
		if err != nil {
			break
		}
		timeOut = blockTime
		height++
	}
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// 7.
	cfg = nodebuilder.DefaultConfig(node.Light)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	lnStore := nodebuilder.MockStore(t, cfg)
	light := sw.MustNewNodeWithStore(node.Light, lnStore)
	require.NoError(t, light.Start(ctx))
	lightClient := getAdminClient(ctx, light, t)

	// 8.
	subCtx, subCancel = context.WithCancel(ctx)
	subscr, err = lightClient.Fraud.Subscribe(subCtx, byzantine.BadEncoding)
	require.NoError(t, err)
	select {
	case p := <-subscr:
		require.Equal(t, 10, int(p.Height()))
		subCancel()
	case <-ctx.Done():
		subCancel()
		t.Fatal("light node did not receive a fraud proof in time")
	}

	// 9.
	fN := sw.MustNewNodeWithStore(node.Full, store)
	err = fN.Start(ctx)
	var fpExist *fraud.ErrFraudExists[*header.ExtendedHeader]
	require.ErrorAs(t, err, &fpExist)

	sw.StopNode(ctx, bridge)
	sw.StopNode(ctx, full)
	sw.StopNode(ctx, light)
	require.NoError(t, <-fillDn)
}
