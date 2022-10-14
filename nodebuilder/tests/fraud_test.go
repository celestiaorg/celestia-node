package tests

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
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
*/
func TestFraudProofBroadcasting(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Millisecond*100))

	bridge := sw.NewBridgeNode(core.WithHeaderConstructFn(header.FraudMaker(t, 20)))

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	cfg := nodebuilder.DefaultConfig(node.Full)
	cfg.Header.TrustedPeers = append(cfg.Header.TrustedPeers, addrs[0].String())
	store := nodebuilder.MockStore(t, cfg)
	full := sw.NewNodeWithStore(node.Full, store)

	err = full.Start(ctx)
	require.NoError(t, err)

	// subscribe to fraud proof before node starts helps
	// to prevent flakiness when fraud proof is propagating before subscribing on it
	subscr, err := full.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)

	p, err := subscr.Proof(ctx)
	require.NoError(t, err)
	require.Equal(t, 20, int(p.Height()))

	// This is an obscure way to check if the Syncer was stopped.
	// If we cannot get a height header within a timeframe it means the syncer was stopped
	// FIXME: Eventually, this should be a check on service registry managing and keeping
	//  lifecycles of each Module.
	syncCtx, syncCancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	_, err = full.HeaderServ.GetByHeight(syncCtx, 100)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	syncCancel()

	require.NoError(t, full.Stop(ctx))
	require.NoError(t, sw.RemoveNode(full, node.Full))

	full = sw.NewNodeWithStore(node.Full, store)

	require.Error(t, full.Start(ctx))
	proofs, err := full.FraudServ.Get(ctx, fraud.BadEncoding)
	require.NoError(t, err)
	require.NotNil(t, proofs)
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
*/
func TestFraudProofSyncing(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Millisecond*300))

	const defaultTimeInterval = time.Second * 5

	cfg := nodebuilder.DefaultConfig(node.Bridge)
	cfg.P2P.Bootstrapper = true
	cfg.P2P.RoutingTableRefreshPeriod = defaultTimeInterval
	cfg.Share.DiscoveryInterval = defaultTimeInterval
	cfg.Share.AdvertiseInterval = defaultTimeInterval

	store := nodebuilder.MockStore(t, cfg)
	bridge := sw.NewNodeWithStore(node.Bridge, store, core.WithHeaderConstructFn(header.FraudMaker(t, 10)))

	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := host.InfoFromHost(bridge.Host)
	addrs, err := peer.AddrInfoToP2pAddrs(addr)
	require.NoError(t, err)

	fullCfg := nodebuilder.DefaultConfig(node.Full)
	fullCfg.Header.TrustedPeers = append(fullCfg.Header.TrustedPeers, addrs[0].String())
	full := sw.NewNodeWithStore(node.Full, nodebuilder.MockStore(t, fullCfg))

	lightCfg := nodebuilder.DefaultConfig(node.Light)
	lightCfg.P2P.RoutingTableRefreshPeriod = defaultTimeInterval
	lightCfg.Share.DiscoveryInterval = defaultTimeInterval
	ln := sw.NewNodeWithStore(node.Light, nodebuilder.MockStore(t, lightCfg),
		nodebuilder.WithBootstrappers([]peer.AddrInfo{*addr}))

	require.NoError(t, full.Start(ctx))
	subsFn, err := full.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)
	defer subsFn.Cancel()
	_, err = subsFn.Proof(ctx)
	require.NoError(t, err)

	require.NoError(t, ln.Start(ctx))
	// internal subscription for the fraud proof is done in order to ensure that light node
	// receives the BEFP.
	subsLn, err := ln.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)
	_, err = subsLn.Proof(ctx)
	require.NoError(t, err)
	subsLn.Cancel()
}
