package tests

import (
	"context"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/app"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/swamp"
	"github.com/celestiaorg/celestia-node/share"
)

func TestStateAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), swamp.DefaultTestTimeout)
	t.Cleanup(cancel)

	sw := swamp.NewSwamp(t, swamp.WithBlockTime(time.Millisecond))
	sw.WaitTillHeight(ctx, 3)

	bridge := sw.NewBridgeNode()
	err := bridge.Start(ctx)
	require.NoError(t, err)

	sw.SetBootstrapper(t, bridge)

	_, err = bridge.HeaderServ.WaitForHeight(ctx, 3)
	require.NoError(t, err)

	// construct a light node with a connection to the core node
	// for state access
	cfg := nodebuilder.DefaultConfig(node.Light)
	cfg.Core.IP, cfg.Core.GRPCPort, err = sw.GetGRPCEndpoint()
	require.NoError(t, err)
	_, cfg.Core.RPCPort, err = sw.GetRPCEndpoint()
	require.NoError(t, err)
	light := sw.NewNodeWithConfig(node.Light, cfg)

	err = light.Start(ctx)
	require.NoError(t, err)

	_, err = light.HeaderServ.WaitForHeight(ctx, 3)
	require.NoError(t, err)

	expectedBal := sdk.NewCoin(app.BondDenom, sdk.NewInt(int64(99999999999999999)))
	bal, err := light.StateServ.Balance(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedBal, *bal)

	randNID := rand.Bytes(10)
	namespace, err := share.NewBlobNamespaceV0(randNID)
	require.NoError(t, err)
	b, err := blob.NewBlobV0(namespace, rand.Bytes(100))
	require.NoError(t, err)
	height, err := light.BlobServ.Submit(ctx, []*blob.Blob{b})
	require.NoError(t, err)
	require.Greater(t, height, uint64(0))
}
