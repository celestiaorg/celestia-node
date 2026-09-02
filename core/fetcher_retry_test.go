package core

import (
	"context"
	"testing"
	"time"

	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type hardErrorSubscriptionClient struct {
	coregrpc.BlockAPIClient
	attempts     chan struct{}
	subscribeErr error
}

func (c *hardErrorSubscriptionClient) SubscribeNewHeights(
	context.Context,
	*coregrpc.SubscribeNewHeightsRequest,
	...grpc.CallOption,
) (coregrpc.BlockAPI_SubscribeNewHeightsClient, error) {
	select {
	case c.attempts <- struct{}{}:
	default:
	}
	if c.subscribeErr != nil {
		return nil, c.subscribeErr
	}
	return hardErrorSubscription{}, nil
}

type hardErrorSubscription struct {
	grpc.ClientStream
}

func (hardErrorSubscription) Recv() (*coregrpc.SubscribeNewHeightsResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "denied")
}

func (hardErrorSubscription) CloseSend() error {
	return nil
}

func TestBlockFetcherSubscriptionHardErrorBacksOff(t *testing.T) {
	tests := []struct {
		name         string
		subscribeErr error
	}{
		{name: "subscribe", subscribeErr: status.Error(codes.PermissionDenied, "denied")},
		{name: "receive"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			client := &hardErrorSubscriptionClient{
				attempts:     make(chan struct{}, 2),
				subscribeErr: test.subscribeErr,
			}
			fetcher := &BlockFetcher{client: client}

			startedAt := time.Now()
			events, err := fetcher.SubscribeNewBlockEvent(ctx)
			require.NoError(t, err)

			select {
			case <-client.attempts:
			case <-time.After(time.Second):
				t.Fatal("subscription was not attempted")
			}

			select {
			case <-client.attempts:
			case _, ok := <-events:
				require.True(t, ok, "subscription stopped instead of retrying")
			case <-time.After(2 * subscriptionRetryInterval):
				t.Fatal("subscription was not retried")
			}
			require.GreaterOrEqual(t, time.Since(startedAt), subscriptionRetryInterval)

			cancel()
			select {
			case _, ok := <-events:
				require.False(t, ok)
			case <-time.After(time.Second):
				t.Fatal("subscription did not stop after cancellation")
			}
		})
	}
}
