package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/testutils"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient(testutils.MockCoreClient())
	require.Nil(t, err)
	t.Cleanup(func() {
		//nolint:errcheck
		client.Stop()
	})
}

func TestClient_GetStatus(t *testing.T) {
	client, err := NewClient(testutils.MockCoreClient())
	require.Nil(t, err)
	t.Cleanup(func() {
		//nolint:errcheck
		client.Stop()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	status, err := client.GetStatus(ctx)
	require.Nil(t, err)
	t.Log(status.NodeInfo)
}

func TestClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	client, err := NewClient(testutils.MockCoreClient())
	require.Nil(t, err)
	t.Cleanup(func() {
		//nolint:errcheck
		client.Stop()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	eventChan, err := client.StartBlockSubscription(ctx)
	require.Nil(t, err)

	for i := 1; i <= 3; i++ {
		<-eventChan
		// check that `GetBlock` works as intended (passing nil to get block at latest height)
		block, err := client.GetBlock(ctx, nil)
		require.Nil(t, err)
		require.Equal(t, int64(i), block.Block.Height)
	}

	// TODO(@Wondertan): For some reason, local client errors with no subscription found, when it exists.
	//  This might be a downstream bug.
	// unsubscribe to event channel
	// err = client.StopBlockSubscription(ctx)
	// require.Nil(t, err)
}
