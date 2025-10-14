package state

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"cosmossdk.io/math"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/v6/app/grpc/gasestimation"
	"github.com/celestiaorg/celestia-app/v6/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v6/test/util/testnode"
)

func TestEstimatorService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mes := setupEstimatorService(t)

	ca, _ := buildAccessor(t, WithEstimatorService(mes.addr))
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ca.Stop(ctx)
	})

	// should estimate gas price using estimator service
	t.Run("tx with default gas price", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig()
		gasPrice, err := ca.estimateGasPrice(ctx, txConfig)
		require.NoError(t, err)
		assert.Equal(t, mes.gasPriceToReturn, gasPrice)
	})
	// should return configured gas price instead of estimated one
	t.Run("tx with manually configured gas price", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig(WithGasPrice(0.005))
		gasPrice, err := ca.estimateGasPrice(ctx, txConfig)
		require.NoError(t, err)
		assert.Equal(t, 0.005, gasPrice)
	})
	// should error out with ErrGasPriceExceedsLimit
	t.Run("estimator exceeds configured max gas price", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig(WithMaxGasPrice(0.0001))
		_, err := ca.estimateGasPrice(ctx, txConfig)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrGasPriceExceedsLimit)
	})
	// should use estimated gas price as it is lower than max gas price
	t.Run("estimated gas price is lower than max gas price, succeeds", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02

		txConfig := NewTxConfig(WithMaxGasPrice(0.1))
		gasPrice, err := ca.estimateGasPrice(ctx, txConfig)
		require.NoError(t, err)
		assert.Equal(t, mes.gasPriceToReturn, gasPrice)
	})
	t.Run("estimate gas price AND usage", func(t *testing.T) {
		mes.gasPriceToReturn = 0.02
		mes.gasUsageToReturn = 10000

		// dummy tx, doesn't matter what's in it
		coins := sdktypes.NewCoins(sdktypes.NewCoin(appconsts.BondDenom, math.NewInt(100)))
		msg := banktypes.NewMsgSend(ca.defaultSignerAddress, ca.defaultSignerAddress, coins)

		testcases := []struct {
			name        string
			txconf      *TxConfig
			expGasPrice float64
			expGasUsage uint64
			errExpected bool
			expectedErr error
		}{
			{
				// should return both the configured gas price and limit
				name:        "configured gas price and limit",
				txconf:      NewTxConfig(WithGasPrice(0.005), WithGas(2000)),
				expGasPrice: 0.005,
				expGasUsage: 2000,
			},
			{
				// should return the configured gas price and estimated gas usage
				name:        "configured gas price only",
				txconf:      NewTxConfig(WithGasPrice(0.005)),
				expGasPrice: 0.005,
				expGasUsage: 10000,
			},
			{
				// should return estimated gas price and gas usage
				name:        "estimate gas price and usage",
				txconf:      NewTxConfig(),
				expGasPrice: 0.02,
				expGasUsage: 10000,
			},
			{
				// should return error as gas price exceeds limit
				name:        "estimate gas price exceeds max gas price",
				txconf:      NewTxConfig(WithMaxGasPrice(0.0001)),
				errExpected: true,
				expectedErr: ErrGasPriceExceedsLimit,
			},
			{
				// should return error as gas price exceeds limit
				name:        "estimate gas price, use configured gas",
				txconf:      NewTxConfig(WithGas(100)),
				expGasPrice: 0.02,
				expGasUsage: 100,
			},
		}

		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				gasPrice, gasUsage, err := ca.estimateGasPriceAndUsage(ctx, tc.txconf, msg)
				if tc.errExpected {
					assert.Error(t, err)
					assert.ErrorIs(t, err, tc.expectedErr)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tc.expGasPrice, gasPrice)
				assert.Equal(t, tc.expGasUsage, gasUsage)
			})
		}
	})
}

type mockEstimatorServer struct {
	*gasestimation.UnimplementedGasEstimatorServer
	srv  *grpc.Server
	addr string

	gasPriceToReturn float64
	gasUsageToReturn uint64
}

func (m *mockEstimatorServer) EstimateGasPrice(
	context.Context,
	*gasestimation.EstimateGasPriceRequest,
) (*gasestimation.EstimateGasPriceResponse, error) {
	return &gasestimation.EstimateGasPriceResponse{
		EstimatedGasPrice: m.gasPriceToReturn,
	}, nil
}

func (m *mockEstimatorServer) EstimateGasPriceAndUsage(
	context.Context,
	*gasestimation.EstimateGasPriceAndUsageRequest,
) (*gasestimation.EstimateGasPriceAndUsageResponse, error) {
	return &gasestimation.EstimateGasPriceAndUsageResponse{
		EstimatedGasPrice: m.gasPriceToReturn,
		EstimatedGasUsed:  m.gasUsageToReturn,
	}, nil
}

func (m *mockEstimatorServer) stop() {
	m.srv.GracefulStop()
}

func setupEstimatorService(t *testing.T) *mockEstimatorServer {
	t.Helper()

	freePort := testnode.MustGetFreePort()
	addr := fmt.Sprintf(":%d", freePort)
	net, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	mes := &mockEstimatorServer{srv: grpcServer, addr: addr}
	gasestimation.RegisterGasEstimatorServer(grpcServer, mes)

	go func() {
		err := grpcServer.Serve(net)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			panic(err)
		}
	}()

	t.Cleanup(mes.stop)
	return mes
}
