package txclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v9/app/grpc/gasestimation"
)

// TestSetupEstimatorConnection verifies the connection is created lazily and is
// usable without blocking on readiness: grpc.NewClient does not dial, so the
// returned conn starts Idle and connects on the first RPC.
func TestSetupEstimatorConnection(t *testing.T) {
	mes := setupEstimatorService(t)
	mes.gasPriceToReturn = 0.02

	conn, err := setupEstimatorConnection(mes.addr, false)
	require.NoError(t, err)
	require.NotNil(t, conn)
	t.Cleanup(func() { _ = conn.Close() })

	// no readiness wait happened, so the conn dials on this first call.
	resp, err := gasestimation.NewGasEstimatorClient(conn).EstimateGasPrice(
		context.Background(), &gasestimation.EstimateGasPriceRequest{},
	)
	require.NoError(t, err)
	require.Equal(t, mes.gasPriceToReturn, resp.EstimatedGasPrice)
}
