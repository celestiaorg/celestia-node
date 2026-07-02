package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// fakeConn stubs the subset of *grpc.ClientConn that waitForCoreConnReady uses.
type fakeConn struct {
	state connectivity.State
}

func (f *fakeConn) Connect() {}

func (f *fakeConn) GetState() connectivity.State { return f.state }

func (f *fakeConn) WaitForStateChange(ctx context.Context, _ connectivity.State) bool {
	// model a never-Ready endpoint: block until the (buffered) context expires
	<-ctx.Done()
	return false
}

// When the endpoint never becomes Ready, waitForCoreConnReady must return a
// descriptive, DeadlineExceeded-wrapped error naming the endpoint — before the
// outer startup deadline so fx does not mask it.
func TestWaitForCoreConnReady_NeverConnects(t *testing.T) {
	conn := &fakeConn{state: connectivity.TransientFailure}

	timeout := utils.StartupErrorBuffer + 300*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	err := waitForCoreConnReady(ctx, conn, "1.2.3.4:9090")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Contains(t, err.Error(), "1.2.3.4:9090")
	assert.Contains(t, err.Error(), "couldn't connect to core endpoint")
	assert.Less(t, elapsed, timeout, "should return via the buffered inner deadline")
	assert.NoError(t, ctx.Err(), "outer startup context must not have expired yet")
}

func TestWaitForCoreConnReady_Ready(t *testing.T) {
	conn := &fakeConn{state: connectivity.Ready}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err := waitForCoreConnReady(ctx, conn, "1.2.3.4:9090")

	require.NoError(t, err)
	assert.Less(t, time.Since(start), time.Second)
}
