package core

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-core/node"
	"github.com/celestiaorg/celestia-core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var newBlockSubscriber = "NewBlock/Events"

func TestEmbeddedClientLifecycle(t *testing.T) {
	client := MockClient()
	require.NoError(t, client.Stop())
}

func TestEmbeddedClient_Status(t *testing.T) {
	client := MockClient()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	status, err := client.Status(ctx)
	require.NoError(t, err)
	assert.NotNil(t, status)

	require.NoError(t, client.Stop())
}

func TestEmbeddedClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	client := MockClient()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	eventChan, err := client.Subscribe(ctx, newBlockSubscriber, types.QueryForEvent(types.EventNewBlock).String())
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		<-eventChan
		// check that `Block` works as intended (passing nil to get block at latest height)
		block, err := client.Block(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, int64(i), block.Block.Height)
	}

	// unsubscribe to event channel
	require.NoError(t, client.Unsubscribe(ctx, newBlockSubscriber, types.QueryForEvent(types.EventNewBlock).String()))
	require.NoError(t, client.Stop())
}

func TestRemoteClientLifecycle(t *testing.T) {
	remote := StartMockNode()

	protocol, ip := getRemoteEndpoint(remote)

	client, err := NewRemote(protocol, ip)
	require.NoError(t, err)

	require.NoError(t, client.Start())
	require.NoError(t, client.Stop())
	require.NoError(t, remote.Stop())
}

func TestRemoteClient_Status(t *testing.T) {
	remote := StartMockNode()
	protocol, ip := getRemoteEndpoint(remote)
	client, err := NewRemote(protocol, ip)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// client must be `started` before performing requests
	err = client.Start()
	require.NoError(t, err)

	status, err := client.Status(ctx)
	require.NoError(t, err)
	assert.NotNil(t, status)

	require.NoError(t, client.Stop())
	require.NoError(t, remote.Stop())
}

func TestRemoteClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	remote := StartMockNode()
	protocol, ip := getRemoteEndpoint(remote)
	client, err := NewRemote(protocol, ip)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// client must be `started` before subscribing to new block events.
	err = client.Start()
	require.NoError(t, err)

	eventChan, err := client.Subscribe(ctx, newBlockSubscriber, types.QueryForEvent(types.EventNewBlock).String())
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		<-eventChan
		// check that `Block` works as intended (passing nil to get block at latest height)
		block, err := client.Block(ctx, nil)
		require.NoError(t, err)
		require.Equal(t, int64(i), block.Block.Height)
	}

	// unsubscribe to event channel
	require.NoError(t, client.Unsubscribe(ctx, newBlockSubscriber, types.QueryForEvent(types.EventNewBlock).String()))
	require.NoError(t, client.Stop())
	require.NoError(t, remote.Stop())
}

// getRemoteEndpoint returns the protocol and ip of the remote node.
func getRemoteEndpoint(remote *node.Node) (string, string) {
	endpoint := remote.Config().RPC.ListenAddress
	// protocol = "tcp"
	protocol, ip := endpoint[:3], endpoint[6:]
	return protocol, ip
}
