package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoteClient_Status(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := StartTestClient(ctx, t)

	status, err := client.Status(ctx)
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestRemoteClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := StartTestClient(ctx, t)

	eventChan, err := client.Subscribe(ctx, newBlockSubscriber, newBlockEventQuery)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		<-eventChan
		// check that `Block` works as intended (passing nil to get block at latest height)
		block, err := client.Block(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, int64(i), block.Block.Height)
	}

	// unsubscribe to event channel
	require.NoError(t, client.Unsubscribe(ctx, newBlockSubscriber, newBlockEventQuery))
}
