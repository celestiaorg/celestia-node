package core

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockFetcher_GetBlock_and_SubscribeNewBlockEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	network := NewNetwork(t, DefaultTestConfig())
	require.NoError(t, network.Start())
	t.Cleanup(func() {
		require.NoError(t, network.Stop())
	})

	fetcher, err := NewBlockFetcher(network.GRPCClient)
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
			assert.Equal(t, newBlockFromChan.Data, block.Data)
			assert.Equal(t, newBlockFromChan.Header, block.Header)
			assert.Equal(t, newBlockFromChan.Commit, block.Commit)
			assert.Equal(t, newBlockFromChan.ValidatorSet, block.ValidatorSet)
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
	client := newTestClient(t, host, port)
	fetcher, err := NewBlockFetcher(client)
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
	t.Cleanup(func() {
		require.NoError(t, tn.Stop())
	})
	select {
	case newBlockFromChan := <-newBlockChan:
		h := newBlockFromChan.Header.Height
		_, err = fetcher.GetSignedBlock(ctx, h)
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for block subscription")
	}
}
