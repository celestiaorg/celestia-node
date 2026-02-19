package core

import (
	"context"
	"testing"
	"time"

	cmthttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"
)

func TestRemoteClient_Status(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)
	cfg := DefaultTestConfig()
	network := NewNetwork(t, cfg)
	require.NoError(t, network.Start())
	t.Cleanup(func() {
		require.NoError(t, network.Stop())
	})

	status, err := network.Client.Status(ctx)
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestRemoteClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	cfg := DefaultTestConfig()
	network := NewNetwork(t, cfg)
	require.NoError(t, network.Start())
	t.Cleanup(func() {
		require.NoError(t, network.Stop())
	})

	wsClient, err := cmthttp.New(cfg.TmConfig.RPC.ListenAddress, "/websocket")
	require.NoError(t, err)

	err = wsClient.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, wsClient.Stop())
	})

	eventChan, err := wsClient.Subscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		select {
		case evt := <-eventChan:
			h := evt.Data.(types.EventDataSignedBlock).Header.Height
			block, err := network.Client.Block(ctx, &h)
			require.NoError(t, err)
			require.GreaterOrEqual(t, block.Block.Height, int64(i))
		case <-ctx.Done():
			t.Fatal("timeout waiting for block event")
		}
	}
	require.NoError(t, wsClient.Unsubscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery))
}
