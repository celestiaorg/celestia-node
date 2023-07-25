package state

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/server/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/config"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/test/util/testnode"
	blobtypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/share"
)

func TestLifecycle(t *testing.T) {
	ca := NewCoreAccessor(nil, nil, "", "", "")
	ctx, cancel := context.WithCancel(context.Background())
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)
	// ensure accessor isn't stopped
	require.False(t, ca.IsStopped(ctx))
	// cancel the top level context (this should not affect the lifecycle of the
	// accessor as it should manage its own internal context)
	cancel()
	// ensure accessor was unaffected by top-level context cancellation
	require.False(t, ca.IsStopped(ctx))
	// stop the accessor
	stopCtx, stopCancel := context.WithCancel(context.Background())
	t.Cleanup(stopCancel)
	err = ca.Stop(stopCtx)
	require.NoError(t, err)
	// ensure accessor is stopped
	require.True(t, ca.IsStopped(ctx))
	// ensure that stopping the accessor again does not return an error
	err = ca.Stop(stopCtx)
	require.NoError(t, err)
}

func TestSubmitPayForBlob(t *testing.T) {
	accounts := []string{"jimmmy", "rob"}
	tmCfg := testnode.DefaultTendermintConfig()
	tmCfg.Consensus.TimeoutCommit = time.Millisecond * 1
	appConf := testnode.DefaultAppConfig()
	appConf.API.Enable = true
	appConf.MinGasPrices = fmt.Sprintf("0.1%s", app.BondDenom)

	cctx, rpcAddr, grpcAddr := NewNetwork(t, testnode.DefaultParams(), tmCfg, appConf, accounts)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signer := blobtypes.NewKeyringSigner(cctx.Keyring, accounts[0], cctx.ChainID)
	ca := NewCoreAccessor(signer, nil, "127.0.0.1", extractPort(rpcAddr), extractPort(grpcAddr))
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)

	ns, err := share.NewBlobNamespaceV0([]byte("namespace"))
	require.NoError(t, err)
	blobbyTheBlob, err := blob.NewBlobV0(ns, []byte("data"))
	require.NoError(t, err)

	minGas, err := ca.queryMinimumGasPrice(ctx)
	require.NoError(t, err)
	fmt.Println(minGas)

	testcases := []struct {
		name   string
		blobs  []*blob.Blob
		fee    sdktypes.Int
		gasLim uint64
		expErr error
	}{
		{
			name:   "empty blobs",
			blobs:  []*blob.Blob{},
			fee:    sdktypes.ZeroInt(),
			gasLim: 0,
			expErr: errors.New("state: no blobs provided"),
		},
		{
			name:   "good blob with user provided gas and fees",
			blobs:  []*blob.Blob{blobbyTheBlob},
			fee:    sdktypes.NewInt(10_000), // roughly 0.12 utia per gas (should be good)
			gasLim: blobtypes.DefaultEstimateGas([]uint32{uint32(len(blobbyTheBlob.Data))}),
			expErr: nil,
		},
		// TODO: add more test cases. The problem right now is that the celestia-app doesn't
		// correctly construct the node (doesn't pass the min gas price) hence the price on
		// everything is zero and we can't actually test the correct behavior
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ca.SubmitPayForBlob(ctx, tc.fee, tc.gasLim, tc.blobs)
			require.Equal(t, tc.expErr, err)
			if err == nil {
				require.EqualValues(t, 0, resp.Code)
			}
		})
	}

}

func extractPort(addr string) string {
	splitStr := strings.Split(addr, ":")
	return splitStr[len(splitStr)-1]
}

// NewNetwork starts a single valiator celestia-app network using the provided
// configurations. Provided accounts will be funded and their keys can be
// accessed in keyring returned client.Context. All rpc, p2p, and grpc addresses
// in the provided configs are overwritten to use open ports. The node can be
// accessed via the returned client.Context or via the returned rpc and grpc
// addresses. Provided genesis options will be applied after all accounts have
// been initialized.
//
// FIXME: this function is copied from celestia-app because it doesn't currently
// support the MinGasPrice query
func NewNetwork(
	t testing.TB,
	cparams *tmproto.ConsensusParams,
	tmCfg *config.Config,
	appCfg *srvconfig.Config,
	accounts []string,
	genesisOpts ...testnode.GenesisOption,
) (cctx testnode.Context, rpcAddr, grpcAddr string) {
	t.Helper()

	tmCfg.RPC.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.GetFreePort())
	tmCfg.P2P.ListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.GetFreePort())
	tmCfg.RPC.GRPCListenAddress = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.GetFreePort())

	genState, kr, err := testnode.DefaultGenesisState(accounts...)
	require.NoError(t, err)

	for _, opt := range genesisOpts {
		genState = opt(genState)
	}

	tmNode, app, cctx, err := testnode.New(t, cparams, tmCfg, false, genState, kr, tmrand.Str(6))
	require.NoError(t, err)

	cctx, stopNode, err := testnode.StartNode(tmNode, cctx)
	require.NoError(t, err)

	// Add the tx service to the gRPC router. We only need to register this
	// service if API or gRPC is enabled, and avoid doing so in the general
	// case, because it spawns a new local tendermint RPC client.
	if (appCfg.API.Enable || appCfg.GRPC.Enable) && tmNode != nil {
		if a, ok := app.(types.ApplicationQueryService); ok {
			a.RegisterNodeService(cctx.Context)
		}
	}

	appCfg.GRPC.Address = fmt.Sprintf("127.0.0.1:%d", testnode.GetFreePort())
	appCfg.API.Address = fmt.Sprintf("tcp://127.0.0.1:%d", testnode.GetFreePort())

	cctx, cleanupGRPC, err := testnode.StartGRPCServer(app, appCfg, cctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		t.Log("tearing down testnode")
		require.NoError(t, stopNode())
		require.NoError(t, cleanupGRPC())
	})

	return cctx, tmCfg.RPC.ListenAddress, appCfg.GRPC.Address
}
