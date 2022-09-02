package nodebuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	coremodule "github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/params"
)

func TestBridge_WithMockedCoreClient(t *testing.T) {
	t.Skip("skipping") // consult https://github.com/celestiaorg/celestia-core/issues/667 for reasoning
	repo := MockStore(t, DefaultConfig(node.Bridge))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := core.StartTestClient(ctx, t)
	node, err := New(node.Bridge, repo,
		coremodule.WithClient(client),
		WithNetwork(params.Private),
	)
	require.NoError(t, err)
	require.NotNil(t, node)
	err = node.Start(ctx)
	require.NoError(t, err)

	err = node.Stop(ctx)
	require.NoError(t, err)
}
