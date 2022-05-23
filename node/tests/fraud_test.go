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
Test-Case: Broadcast Fraud Proof to different nodes
Pre-Requisites:
- CoreClient is started by swamp
- CoreClient has generated 2 blocks
Steps:
1. Create a Bridge Node(BN) with broken block an height 10
2. Start a BN
3. Create a Full Node(FN) with a connection to BN as a trusted peer
4. Start a FN
5. Check FN is not synced to 11.
11 is not available because DASer will be stopped after receiving BEFP.
*/
func TestFraudProofBroadcasting(t *testing.T) {
	sw := swamp.NewSwamp(t, swamp.WithBlockInterval(time.Millisecond*100))

	bridge := sw.NewBridgeNode(node.WithHeaderConstructFn(header.FraudMaker(t, 10)))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	err := bridge.Start(ctx)
	require.NoError(t, err)
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(bridge.Host))
	require.NoError(t, err)
	full := sw.NewFullNode(node.WithTrustedPeers(addrs[0].String()))

	err = full.Start(ctx)
	require.NoError(t, err)

	subscr, err := full.FraudServ.Subscribe(fraud.BadEncoding)
	require.NoError(t, err)
	_, err = subscr.Proof(ctx)
	require.NoError(t, err)

	// Since GetByHeight is a blocking operation for headers that is not received, we
	// should set a tiomeout because all daser/syncer are stopped at this point
	newCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	t.Cleanup(cancel)
	_, err = full.HeaderServ.GetByHeight(newCtx, 15)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
