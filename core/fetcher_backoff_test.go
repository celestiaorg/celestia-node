package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// refusingBlockAPIClient refuses every new-height subscription at once and
// counts the attempts.
type refusingBlockAPIClient struct {
	coregrpc.BlockAPIClient
	subscribeAttempts atomic.Int64
}

func (c *refusingBlockAPIClient) SubscribeNewHeights(
	context.Context,
	*coregrpc.SubscribeNewHeightsRequest,
	...grpc.CallOption,
) (coregrpc.BlockAPI_SubscribeNewHeightsClient, error) {
	c.subscribeAttempts.Add(1)
	return nil, status.Error(codes.ResourceExhausted, "no subscription slots left")
}

// TestBlockFetcher_SubscribeNewBlockEvent_BacksOff ensures that the fetcher
// waits between retries when the endpoint refuses the subscription at once.
func TestBlockFetcher_SubscribeNewBlockEvent_BacksOff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client := &refusingBlockAPIClient{}
	fetcher := &BlockFetcher{client: client}

	eventCh, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	const observationTime = time.Second
	time.Sleep(observationTime)
	cancel()

	// The waits are at least 50ms, 100ms, 200ms, 400ms and 800ms, so no more
	// than 5 attempts fit into the observation time. Leave room for slow timers.
	const maxExpectedAttempts = 10
	attempts := client.subscribeAttempts.Load()
	require.Positive(t, attempts)
	require.LessOrEqual(t, attempts, int64(maxExpectedAttempts))

	// The subscription goroutine stops and closes the channel on cancel.
	select {
	case _, open := <-eventCh:
		require.False(t, open)
	case <-time.After(5 * time.Second):
		t.Fatal("event channel was not closed after cancel")
	}
}
