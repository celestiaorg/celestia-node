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

// TestSubscribeRetryDelay ensures that the wait doubles on every failure in a
// row, stops growing at the maximum and starts over after a reset.
func TestSubscribeRetryDelay(t *testing.T) {
	retryDelay := newSubscribeRetryDelay()

	require.Equal(t, subscribeRetryInitialDelay, retryDelay.next())
	require.Equal(t, 2*subscribeRetryInitialDelay, retryDelay.next())
	require.Equal(t, 4*subscribeRetryInitialDelay, retryDelay.next())

	// Enough failures in a row to reach the maximum.
	for range 20 {
		retryDelay.next()
	}
	require.Equal(t, subscribeRetryMaxDelay, retryDelay.next())
	require.Equal(t, subscribeRetryMaxDelay, retryDelay.next())

	retryDelay.reset()
	require.Equal(t, subscribeRetryInitialDelay, retryDelay.next())
}

func TestWithJitter(t *testing.T) {
	const delay = time.Second
	for range 1000 {
		jittered := withJitter(delay)
		require.GreaterOrEqual(t, jittered, delay/2)
		require.LessOrEqual(t, jittered, delay)
	}
}

// oneHeightStream delivers one height and then fails.
type oneHeightStream struct {
	coregrpc.BlockAPI_SubscribeNewHeightsClient
	delivered bool
}

func (s *oneHeightStream) Recv() (*coregrpc.SubscribeNewHeightsResponse, error) {
	if s.delivered {
		return nil, status.Error(codes.Unavailable, "stream closed")
	}
	s.delivered = true
	return &coregrpc.SubscribeNewHeightsResponse{Height: 7}, nil
}

func (s *oneHeightStream) CloseSend() error { return nil }

// oneHeightBlockAPIClient hands out a stream that delivers one height.
type oneHeightBlockAPIClient struct {
	coregrpc.BlockAPIClient
}

func (oneHeightBlockAPIClient) SubscribeNewHeights(
	context.Context,
	*coregrpc.SubscribeNewHeightsRequest,
	...grpc.CallOption,
) (coregrpc.BlockAPI_SubscribeNewHeightsClient, error) {
	return &oneHeightStream{}, nil
}

// TestBlockFetcher_SubscribeAndReceive_CountsHeights ensures that a
// subscription that delivered a height before failing reports it, which is
// what resets the retry wait.
func TestBlockFetcher_SubscribeAndReceive_CountsHeights(t *testing.T) {
	fetcher := &BlockFetcher{client: oneHeightBlockAPIClient{}}
	eventCh := make(chan BlockEvent, 1)

	receivedHeights, err := fetcher.subscribeAndReceive(context.Background(), eventCh)
	require.Error(t, err)
	require.Equal(t, 1, receivedHeights)
	require.Equal(t, int64(7), (<-eventCh).Height)

	refusingFetcher := &BlockFetcher{client: &refusingBlockAPIClient{}}
	receivedHeights, err = refusingFetcher.subscribeAndReceive(context.Background(), eventCh)
	require.Error(t, err)
	require.Zero(t, receivedHeights)
}
