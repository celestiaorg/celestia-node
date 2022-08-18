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
	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/tests/swamp"
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

	bridge := sw.NewBridgeNode(node.WithHeaderConstructFn(header.FraudMaker(t, 20)))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)

	store := node.MockStore(t, node.DefaultConfig(node.Full))
	full := sw.NewNodeWithStore(node.Full, store, node.WithTrustedPeers(addrs[0].String()))

	err = full.Start(ctx)
	require.NoError(t, err)

	// subscribe to fraud proof before node starts helps
	// to prevent flakiness when fraud proof is propagating before subscribing on it
	subscr, err := full.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)

	_, err = subscr.Proof(ctx)
	require.NoError(t, err)

	// Since GetByHeight is a blocking operation for headers that is not received, we
	// should set a timeout because all daser/syncer are stopped at this point
	newCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	// rework this after https://github.com/celestiaorg/celestia-node/issues/427
	t.Cleanup(cancel)

	_, err = full.HeaderServ.GetByHeight(newCtx, 25)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	require.NoError(t, full.Stop(ctx))
	require.NoError(t, sw.RemoveNode(full, node.Full))

	full = sw.NewNodeWithStore(node.Full, store, node.WithTrustedPeers(addrs[0].String()))
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
	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Millisecond*100))
	cfg := node.DefaultConfig(node.Bridge)
	cfg.P2P.Bootstrapper = true
	const defaultTimeInterval = time.Second * 5
	bridge := sw.NewBridgeNode(
		node.WithConfig(cfg),
		node.WithRefreshRoutingTablePeriod(defaultTimeInterval),
		node.WithDiscoveryInterval(defaultTimeInterval),
		node.WithAdvertiseInterval(defaultTimeInterval),
		node.WithHeaderConstructFn(header.FraudMaker(t, 10)),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addr := host.InfoFromHost(bridge.Host)
	addrs, err := peer.AddrInfoToP2pAddrs(addr)
	require.NoError(t, err)

	full := sw.NewFullNode(
		node.WithTrustedPeers(addrs[0].String()),
		node.WithRefreshRoutingTablePeriod(defaultTimeInterval),
		node.WithAdvertiseInterval(defaultTimeInterval),
	)

	ln := sw.NewLightNode(
		node.WithBootstrappers([]peer.AddrInfo{*addr}),
		node.WithRefreshRoutingTablePeriod(defaultTimeInterval),
		node.WithDiscoveryInterval(defaultTimeInterval),
	)

	require.NoError(t, full.Start(ctx))
	subsFn, err := full.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)
	defer subsFn.Cancel()
	_, err = subsFn.Proof(ctx)
	require.NoError(t, err)

	require.NoError(t, ln.Start(ctx))
	subsLn, err := ln.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)
	_, err = subsLn.Proof(ctx)
	require.NoError(t, err)
	subsLn.Cancel()
}
