package rpc

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-core/node"
	"github.com/celestiaorg/celestia-node/testutils"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	_, backgroundNode := newClient(t)
	//nolint:errcheck
	backgroundNode.Stop()
}

func TestClient_GetStatus(t *testing.T) {
	client, backgroundNode := newClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	//nolint:errcheck
	t.Cleanup(func() {
		backgroundNode.Stop()
		cancel()
	})

	status, err := client.GetStatus(ctx)
	require.Nil(t, err)
	t.Log(status.NodeInfo)
}

func TestClient_StartBlockSubscription_And_GetBlock(t *testing.T) {
	client, backgroundNode := newClient(t)

	ctx, cancel := context.WithCancel(context.Background())
	//nolint:errcheck
	t.Cleanup(func() {
		backgroundNode.Stop()
		cancel()
	})

	// make 3 blocks
	err := client.Start()
	require.Nil(t, err)

	eventChan, err := client.StartBlockSubscription(ctx)
	require.Nil(t, err)

	for i := 1; i <= 3; i++ {
		<-eventChan
		// check that `GetBlock` works as intended (passing nil to get block at latest height)
		block, err := client.GetBlock(ctx, nil)
		require.Nil(t, err)
		require.Equal(t, int64(i), block.Block.Height)
	}
	// unsubscribe to event channel
	err = client.StopBlockSubscription(ctx)
	require.Nil(t, err)
}

func newClient(t *testing.T) (*Client, *node.Node) {
	backgroundNode, protocol, ip := testutils.StartMockCoreNode()
	t.Cleanup(func() {
		//nolint:errcheck
		backgroundNode.Stop()
	})

	client, err := NewClient(protocol, ip)
	require.Nil(t, err)
	return client, backgroundNode
}
