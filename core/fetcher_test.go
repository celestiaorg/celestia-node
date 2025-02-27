package core

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
)

// TestRoundRobinClientSelection ensures that the round-robin client selection
// mechanism works as expected, rotating through available clients.
func TestRoundRobinClientSelection(t *testing.T) {
	// Create a BlockFetcher with mock clients to test the round-robin behavior
	clients := []coregrpc.BlockAPIClient{
		&mockBlockAPIClient{id: 1},
		&mockBlockAPIClient{id: 2},
		&mockBlockAPIClient{id: 3},
	}

	fetcher := &BlockFetcher{
		clients:       clients,
		currentClient: 0,
	}

	// The first call should return client 0 (id: 1)
	client1 := fetcher.getCurrentClient()
	mockClient1, ok := client1.(*mockBlockAPIClient)
	require.True(t, ok, "Expected mockBlockAPIClient")
	assert.Equal(t, 1, mockClient1.id)

	// The second call should return client 1 (id: 2)
	client2 := fetcher.getCurrentClient()
	mockClient2, ok := client2.(*mockBlockAPIClient)
	require.True(t, ok, "Expected mockBlockAPIClient")
	assert.Equal(t, 2, mockClient2.id)

	// The third call should return client 2 (id: 3)
	client3 := fetcher.getCurrentClient()
	mockClient3, ok := client3.(*mockBlockAPIClient)
	require.True(t, ok, "Expected mockBlockAPIClient")
	assert.Equal(t, 3, mockClient3.id)

	// The fourth call should wrap around and return client 0 (id: 1) again
	client4 := fetcher.getCurrentClient()
	mockClient4, ok := client4.(*mockBlockAPIClient)
	require.True(t, ok, "Expected mockBlockAPIClient")
	assert.Equal(t, 1, mockClient4.id)
}

// mockBlockAPIClient is a mock implementation of coregrpc.BlockAPIClient for testing
type mockBlockAPIClient struct {
	id int
	coregrpc.BlockAPIClient
}

func TestBlockFetcher_GetBlock_and_SubscribeNewBlockEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	host, port, err := net.SplitHostPort(StartTestNode(t).GRPCClient.Target())
	require.NoError(t, err)
	fetcher, err := newTestBlockFetcher(t, host, port)
	require.NoError(t, err)
	// generate some blocks
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	for i := 1; i < 3; i++ {
		select {
		case newBlockFromChan := <-newBlockChan:
			h := newBlockFromChan.Header.Height
			block, err := fetcher.GetSignedBlock(ctx, h)
			require.NoError(t, err)
			assert.Equal(t, newBlockFromChan.Data, *block.Data)
			assert.Equal(t, newBlockFromChan.Header, *block.Header)
			assert.Equal(t, newBlockFromChan.Commit, *block.Commit)
			assert.Equal(t, newBlockFromChan.ValidatorSet, *block.ValidatorSet)
			require.GreaterOrEqual(t, newBlockFromChan.Header.Height, int64(i))
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}
	}
}

// TestFetcher_Resubscription ensures that subscription will not stuck in case
// gRPC server was stopped.
func TestFetcher_Resubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	// run new consensus node
	cfg := DefaultTestConfig()
	tn := NewNetwork(t, cfg)
	require.NoError(t, tn.Start())
	host, port, err := net.SplitHostPort(tn.GRPCClient.Target())
	require.NoError(t, err)
	fetcher, err := newTestBlockFetcher(t, host, port)
	require.NoError(t, err)

	// subscribe to the channel to get new blocks
	// and try to get one block
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	select {
	case newBlockFromChan := <-newBlockChan:
		h := newBlockFromChan.Header.Height
		_, err = fetcher.GetSignedBlock(ctx, h)
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for block subscription")
	}
	// stop the consensus node and wait some time to ensure that the subscription is stuck
	// and there is no connection with the consensus node.
	require.NoError(t, tn.Stop())

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(waitCancel)
	select {
	case <-newBlockChan:
		t.Fatal("blocks received after stopping")
	case <-ctx.Done():
		t.Fatal("test finishes")
	case <-waitCtx.Done():
	}

	// start new consensus node(some components in app can't be restarted)
	// on the same address and listen for the new blocks
	tn = NewNetwork(t, cfg)
	require.NoError(t, tn.Start())
	select {
	case newBlockFromChan := <-newBlockChan:
		h := newBlockFromChan.Header.Height
		_, err = fetcher.GetSignedBlock(ctx, h)
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for block subscription")
	}
	require.NoError(t, tn.Stop())
}
