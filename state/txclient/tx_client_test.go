package txclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

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

// TestSetupClient_ClosesEstimatorConnOnFailure ensures the estimator connection
// opened during setup is closed when user.SetupTxClient fails. On that path the
// TxClient is never assigned, so Stop() won't run to release the connection;
// without the explicit close it would leak its transport and resolver/balancer
// goroutines.
//
// We point the core connection at the estimator mock, which does not implement
// the node-info service, so SetupTxClient's first RPC fails fast. The estimator
// connection handed to setupClient is captured via newEstimatorConnection and
// asserted to be in the Shutdown state (i.e. Close was called). Reverting the
// close leaves it Idle and turns this test red — deterministically, without
// counting goroutines.
func TestSetupClient_ClosesEstimatorConnOnFailure(t *testing.T) {
	mes := setupEstimatorService(t)

	coreConn, err := grpc.NewClient(mes.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = coreConn.Close() })

	var captured *grpc.ClientConn
	orig := newEstimatorConnection
	newEstimatorConnection = func(addr string, tlsEnabled bool) (*grpc.ClientConn, error) {
		conn, connErr := orig(addr, tlsEnabled)
		captured = conn
		return conn, connErr
	}
	t.Cleanup(func() { newEstimatorConnection = orig })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	c := &TxClient{
		ctx:                  ctx,
		cancel:               cancel,
		coreConns:            []*grpc.ClientConn{coreConn},
		estimatorServiceAddr: mes.addr,
	}

	err = c.setupClient()
	require.Error(t, err)
	require.Nil(t, c.client)
	// estimatorConn must not be retained on the failure path.
	require.Nil(t, c.estimatorConn)

	require.NotNil(t, captured, "estimator connection was never opened")
	require.Equal(t, connectivity.Shutdown, captured.GetState(),
		"estimator connection was not closed after SetupTxClient failure")
}
