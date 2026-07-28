package core

import (
	"context"
	"errors"
	"testing"
	"time"

	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/store"
)

// gappyBlockAPIClient fails BlockByHeight for one height instantly and delays every other,
// so the mid-range failure lands while the lower fetches are still in flight.
type gappyBlockAPIClient struct {
	coregrpc.BlockAPIClient
	failHeight int64
	delay      time.Duration
}

func (g *gappyBlockAPIClient) BlockByHeight(
	ctx context.Context,
	in *coregrpc.BlockByHeightRequest,
	opts ...grpc.CallOption,
) (coregrpc.BlockAPI_BlockByHeightClient, error) {
	if in.Height == g.failHeight {
		return nil, errors.New("gappy: height unavailable")
	}
	time.Sleep(g.delay)
	return g.BlockAPIClient.BlockByHeight(ctx, in, opts...)
}

// TestExchange_ReturnsLargestContiguousRange verifies that a mid-range failure does not
// cancel the lower in-flight fetches, so the full contiguous prefix below it is returned.
func TestExchange_ReturnsLargestContiguousRange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const (
		to         = 15
		failHeight = 10
	)

	cfg := DefaultTestConfig()
	_, network := createCoreFetcher(t, cfg)
	_, err := network.WaitForHeightWithTimeout(to, 20*time.Second)
	require.NoError(t, err)

	fetcher := &BlockFetcher{client: &gappyBlockAPIClient{
		BlockAPIClient: coregrpc.NewBlockAPIClient(network.GRPCClient),
		failHeight:     failHeight,
		delay:          100 * time.Millisecond,
	}}

	st, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	ce, err := NewExchange(fetcher, st, header.MakeExtendedHeader)
	require.NoError(t, err)

	from, err := ce.GetByHeight(ctx, 1)
	require.NoError(t, err)

	headers, err := ce.GetRangeByHeight(ctx, from, to)
	require.NoError(t, err)

	require.Len(t, headers, failHeight-2)
	for i, h := range headers {
		require.Equal(t, uint64(2+i), h.Height())
	}
}
