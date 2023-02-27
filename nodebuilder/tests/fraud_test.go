package tests

import (
	"context"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
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
Note: 15 is not available because DASer will be stopped before reaching this height due to receiving a fraud proof.
Another note: this test disables share exchange to speed up test results.
*/
func TestFraudProofBroadcasting(t *testing.T) {
	const (
		blocks = 15
		bsize  = 2
		btime  = time.Millisecond * 300
	)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	cfg.Share.UseShareExchange = false
	bridge := sw.NewNodeWithConfig(
		node.Bridge,
		cfg,
		core.WithHeaderConstructFn(headertest.FraudMaker(t, 10, mdutils.Bserv())),
	)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg = nodebuilder.DefaultConfig(node.Full)
	cfg.Share.UseShareExchange = false
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	store := nodebuilder.MockStore(t, cfg)
	full := sw.NewNodeWithStore(node.Full, store)

	err = full.Start(ctx)
	require.NoError(t, err)

	// subscribe to fraud proof before node starts helps
	// to prevent flakiness when fraud proof is propagating before subscribing on it
	subscr, err := full.FraudServ.Subscribe(ctx, byzantine.BadEncoding)
	require.NoError(t, err)

	p := <-subscr
	require.Equal(t, 10, int(p.Height()))

	// This is an obscure way to check if the Syncer was stopped.
	// If we cannot get a height header within a timeframe it means the syncer was stopped
	// FIXME: Eventually, this should be a check on service registry managing and keeping
	//  lifecycles of each Module.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), btime)
	_, err = full.HeaderServ.GetByHeight(syncCtx, 100)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	syncCancel()

	require.NoError(t, full.Stop(ctx))
	require.NoError(t, sw.RemoveNode(full, node.Full))

	full = sw.NewNodeWithStore(node.Full, store)

	require.Error(t, full.Start(ctx))
	proofs, err := full.FraudServ.Get(ctx, byzantine.BadEncoding)
	require.NoError(t, err)
	require.NotNil(t, proofs)
	require.NoError(t, <-fillDn)
}

/*
Test-Case: Light node receives a fraud proof using Fraud Sync
Pre-Requisites:
- CoreClient is started by swamp.
Steps:
1. Create a Bridge Node(BN) with broken extended header at height 10.
2. Start a BN.
3. Create a Full Node(FN) with a connection to BN as a trusted peer.
4. Start a FN.
5. Subscribe to a fraud proof and wait when it will be received.
6. Start LN once a fraud proof is received and verified by FN.
7. Wait until LN will be connected to FN and fetch a fraud proof.
Note: this test disables share exchange to speed up test results.
*/
func TestFraudProofSyncing(t *testing.T) {
	const (
		blocks = 15
		bsize  = 2
		btime  = time.Millisecond * 300
	)
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(btime))
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	fillDn := swamp.FillBlocks(ctx, sw.ClientContext, sw.Accounts, bsize, blocks)
	cfg := nodebuilder.DefaultConfig(node.Bridge)
	cfg.Share.UseShareExchange = false
	store := nodebuilder.MockStore(t, cfg)
	bridge := sw.NewNodeWithStore(
		node.Bridge,
		store,
		core.WithHeaderConstructFn(headertest.FraudMaker(t, 10, mdutils.Bserv())),
	)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := host.InfoFromHost(bridge.Host)
	addrs, err := peer.AddrInfoToP2pAddrs(addr)
	require.NoError(t, err)

	fullCfg := nodebuilder.DefaultConfig(node.Full)
	fullCfg.Share.UseShareExchange = false
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	full := sw.NewNodeWithStore(node.Full, nodebuilder.MockStore(t, fullCfg))

	lightCfg := nodebuilder.DefaultConfig(node.Light)
	lightCfg.Header.TrustedPeers = append(lightCfg.Header.TrustedPeers, addrs[0].String())
	ln := sw.NewNodeWithStore(node.Light, nodebuilder.MockStore(t, lightCfg))
	require.NoError(t, full.Start(ctx))

	subsFN, err := full.FraudServ.Subscribe(ctx, byzantine.BadEncoding)
	require.NoError(t, err)

	select {
	case <-subsFN:
	case <-ctx.Done():
		t.Fatal("full node didn't get FP in time")
	}

	// start LN to enforce syncing logic, not the PubSub's broadcasting
	err = ln.Start(ctx)
	require.NoError(t, err)

	// internal subscription for the fraud proof is done in order to ensure that light node
	// receives the BEFP.
	subsLN, err := ln.FraudServ.Subscribe(ctx, byzantine.BadEncoding)
	require.NoError(t, err)

	// ensure that the full and light node are connected to speed up test
	// alternatively, they would discover each other
	err = ln.Host.Connect(ctx, *host.InfoFromHost(full.Host))
	require.NoError(t, err)

	select {
	case <-subsLN:
	case <-ctx.Done():
		t.Fatal("light node didn't get FP in time")
	}
	require.NoError(t, <-fillDn)
}
