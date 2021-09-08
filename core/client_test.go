package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientLifecycle(t *testing.T) {
	client := MockClient()
	err := client.Stop()
	require.NoError(t, err)
}

func TestClient_Status(t *testing.T) {
	client := MockClient()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	status, err := client.Status(ctx)
	require.Nil(t, err)
	assert.NotNil(t, status)

	err = client.Stop()
	require.NoError(t, err)
}

func TestClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	client := MockClient()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	eventChan, err := client.Subscribe(ctx, )
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
