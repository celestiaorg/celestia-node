package txclient

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSetupEstimatorConnection_Success verifies the happy path returns a
// connection that has actually reached Ready and is the caller's to close.
func TestSetupEstimatorConnection_Success(t *testing.T) {
	mes := setupEstimatorService(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := setupEstimatorConnection(ctx, mes.addr, false)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}

// TestSetupEstimatorConnection_UnreachableReturnsError verifies that an
// unreachable endpoint surfaces an error once the context deadline fires,
// rather than silently returning a not-yet-connected conn.
//
// Before the fix, WaitForStateChange(ctx, Ready) returned immediately (a fresh
// conn is in Idle/Connecting, already != Ready), so setupEstimatorConnection
// returned a non-Ready conn with a nil error and never honored the deadline.
func TestSetupEstimatorConnection_UnreachableReturnsError(t *testing.T) {
	// RFC 5737 TEST-NET-1: guaranteed non-routable, so Ready is never reached.
	const blackhole = "192.0.2.1:65000"

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	conn, err := setupEstimatorConnection(ctx, blackhole, false)
	require.Error(t, err)
	require.Nil(t, conn)
}

// TestSetupEstimatorConnection_NoLeakOnFailure guards against the gRPC
// connection leak on the failure path: setupEstimatorConnection used to return
// an error without closing the already-created *grpc.ClientConn, leaking its
// resolver/balancer goroutines and transport. setupClient retries on each
// submit while the client is not yet initialized, so an unreachable estimator
// would accumulate one leaked conn per attempt.
//
// We assert that repeated failed connections do not grow the goroutine count,
// which they would if each conn were left open.
func TestSetupEstimatorConnection_NoLeakOnFailure(t *testing.T) {
	const blackhole = "192.0.2.1:65000"

	connectOnce := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		conn, err := setupEstimatorConnection(ctx, blackhole, false)
		require.Error(t, err)
		require.Nil(t, conn)
	}

	// Warm up so grpc/runtime one-time goroutines exist before the baseline,
	// otherwise the first iteration inflates the count.
	connectOnce()
	stabilizeGoroutines()
	baseline := runtime.NumGoroutine()

	const attempts = 10
	for range attempts {
		connectOnce()
	}
	stabilizeGoroutines()

	// A real leak would add several goroutines per attempt, well above this
	// slack for transient scheduler/runtime goroutines.
	const slack = 5
	got := runtime.NumGoroutine()
	require.LessOrEqualf(t, got, baseline+slack,
		"goroutine count grew from %d to %d after %d failed connections: "+
			"setupEstimatorConnection likely leaks the *grpc.ClientConn on the failure path",
		baseline, got, attempts)
}

// stabilizeGoroutines gives background goroutines a chance to wind down so the
// goroutine-count reading is stable.
func stabilizeGoroutines() {
	for range 10 {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
}
