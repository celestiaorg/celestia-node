package node

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/celestiaorg/celestia-node/params"

	"github.com/celestiaorg/celestia-node/core"
)

func TestNewBridgeAndLifecycle(t *testing.T) {
	node := TestNode(t, Bridge)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotZero(t, node.Type)
	require.NotNil(t, node.Host)
	require.NotNil(t, node.CoreClient)
	require.NotNil(t, node.HeaderServ)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := node.Start(ctx)
	require.NoError(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Stop(ctx)
	require.NoError(t, err)
}

func TestBridge_WithMockedCoreClient(t *testing.T) {
	t.Skip("skipping") // consult https://github.com/celestiaorg/celestia-core/issues/667 for reasoning
	repo := MockStore(t, DefaultConfig(Bridge))

	_, client := core.StartTestClient(t)
	node, err := New(Bridge, repo, WithCoreClient(client), WithNetwork(params.Private))
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.True(t, node.CoreClient.IsRunning())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	err = node.Stop(ctx)
	require.NoError(t, err)
}
