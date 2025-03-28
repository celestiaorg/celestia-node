package state

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-app/v3/app/grpc/gasestimation"
	"github.com/celestiaorg/celestia-app/v3/test/util/testnode"
)

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

	freePort, err := testnode.GetFreePort()
	require.NoError(t, err)
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
