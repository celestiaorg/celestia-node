package state

import (
	"context"
	"testing"
	"time"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// fakeServiceClient stubs tmservice.ServiceClient, overriding only GetNodeInfo.
// The embedded nil interface panics if any other method is called, which is fine
// because waitForAppReady only calls GetNodeInfo.
type fakeServiceClient struct {
	tmservice.ServiceClient
	getNodeInfo func(context.Context) (*tmservice.GetNodeInfoResponse, error)
}

func (f fakeServiceClient) GetNodeInfo(
	ctx context.Context,
	_ *tmservice.GetNodeInfoRequest,
	_ ...grpc.CallOption,
) (*tmservice.GetNodeInfoResponse, error) {
	return f.getNodeInfo(ctx)
}

// When the endpoint is reachable but the app keeps replying "please wait for first
// block", waitForAppReady must return a descriptive, DeadlineExceeded-wrapped error
// naming the endpoint — and return before the outer startup deadline so fx does not
// mask it with a bare context.DeadlineExceeded.
func TestWaitForAppReady_AppNotProducingBlocks(t *testing.T) {
	cli := fakeServiceClient{
		getNodeInfo: func(context.Context) (*tmservice.GetNodeInfoResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "please wait for first block")
		},
	}

	// Outer deadline is StartupErrorBuffer + a small margin, so the buffered inner
	// deadline is positive but small and the test stays fast.
	timeout := utils.StartupErrorBuffer + 300*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	_, err := waitForAppReady(ctx, cli, "1.2.3.4:9090")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Contains(t, err.Error(), "1.2.3.4:9090")
	assert.Contains(t, err.Error(), "not producing blocks")
	assert.Less(t, elapsed, timeout, "should return via the buffered inner deadline")
	assert.NoError(t, ctx.Err(), "outer startup context must not have expired yet")
}

// Errors that are not the "please wait for first block" signal (dial refused, wrong
// network, auth) must still fail fast, unchanged.
func TestWaitForAppReady_FastFailsOnOtherErrors(t *testing.T) {
	wantErr := status.Error(codes.Unavailable, "connection refused")
	cli := fakeServiceClient{
		getNodeInfo: func(context.Context) (*tmservice.GetNodeInfoResponse, error) {
			return nil, wantErr
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	_, err := waitForAppReady(ctx, cli, "1.2.3.4:9090")

	assert.ErrorIs(t, err, wantErr)
	assert.Less(t, time.Since(start), time.Second, "non-'not ready' errors must not retry")
}
