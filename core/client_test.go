package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"
)

func TestRemoteClient_Status(t *testing.T) {
	client := StartTestNode(t).Client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
	t.Cleanup(cancel)
	status, err := client.Status(ctx)
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestRemoteClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	client := StartTestNode(t).Client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
	t.Cleanup(cancel)
	eventChan, err := client.Subscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery)
	require.NoError(t, err)
	for i := 1; i <= 3; i++ {
		select {
		case evt := <-eventChan:
			h := evt.Data.(types.EventDataSignedBlock).Header.Height
			block, err := client.Block(ctx, &h)
			require.NoError(t, err)
			require.GreaterOrEqual(t, block.Block.Height, int64(i))
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}
	}
	// unsubscribe to event channel
	require.NoError(t, client.Unsubscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery))
}
