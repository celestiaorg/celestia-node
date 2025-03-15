package state

import (
	"context"
	"errors"
	"net"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/app/encoding"
	"github.com/celestiaorg/celestia-app/v3/app/grpc/gasestimation"
	"github.com/celestiaorg/celestia-app/v3/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v3/pkg/user"
	"github.com/celestiaorg/celestia-app/v3/test/util/testfactory"
	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
)

type mockEstimatorServer struct {
	*gasestimation.UnimplementedGasEstimatorServer
	conn *grpc.Server
}

func (m *mockEstimatorServer) EstimateGasPriceAndUsage(
	context.Context,
	*gasestimation.EstimateGasPriceAndUsageRequest,
) (*gasestimation.EstimateGasPriceAndUsageResponse, error) {
	return &gasestimation.EstimateGasPriceAndUsageResponse{
		EstimatedGasPrice: 0.02,
		EstimatedGasUsed:  70000,
	}, nil
}

func setupServer(t *testing.T) *mockEstimatorServer {
	t.Helper()
	net, err := net.Listen("tcp", ":9090")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	gasestimation.RegisterGasEstimatorServer(grpcServer, &mockEstimatorServer{})

	go func() {
		err := grpcServer.Serve(net)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			panic(err)
		}
	}()

	return &mockEstimatorServer{conn: grpcServer}
}

func (m *mockEstimatorServer) stop() {
	m.conn.GracefulStop()
}

func TestEstimator(t *testing.T) {
	server := setupServer(t)
	t.Cleanup(server.stop)
	target := "0.0.0.0:9090"

	accountName := "test"
	config := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	keyring := testfactory.TestKeyring(config.Codec, accountName)
	account := user.NewAccount(accountName, 0, 0)
	signer, err := user.NewSigner(keyring, config.TxConfig, "test", appconsts.LatestVersion, account)
	require.NoError(t, err)

	msgSend := banktypes.NewMsgSend(
		account.Address(),
		testnode.RandomAddress().(sdk.AccAddress),
		sdk.NewCoins(sdk.NewInt64Coin(appconsts.BondDenom, 10)),
	)
	rawTx, err := signer.CreateTx([]sdk.Msg{msgSend})
	require.NoError(t, err)

	defaultConn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	estimator := estimator{estimatorAddress: target}

	testCases := []struct {
		name string
		doFn func()
	}{
		{
			name: "query from estimator endpoint",
			doFn: func() {
				require.NoError(t, estimator.Start(context.TODO()))
			},
		},
		{
			name: "query from default estimator endpoint",
			doFn: func() {
				require.NoError(t, estimator.Stop(context.TODO()))
				estimator.defaultClientConn = defaultConn
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.doFn()
			gasPrice, gas, err := estimator.queryGasUsedAndPrice(context.Background(), signer, TxPriorityMedium, rawTx)
			require.NoError(t, err)
			assert.Greater(t, gasPrice, float64(0))
			assert.Greater(t, gas, uint64(0))
		})
	}
}
