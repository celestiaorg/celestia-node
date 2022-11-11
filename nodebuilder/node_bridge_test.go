package nodebuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/das"
	coremodule "github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func TestBridge_WithMockedCoreClient(t *testing.T) {
	t.Skip("skipping") // consult https://github.com/celestiaorg/celestia-core/issues/667 for reasoning
	repo := MockStore(t, DefaultConfig(node.Bridge))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := core.StartTestClient(ctx, t)
	node, err := New(node.Bridge, p2p.Private, repo, coremodule.WithClient(client))
	require.NoError(t, err)
	require.NotNil(t, node)
	err = node.Start(ctx)
	require.NoError(t, err)

	err = node.Stop(ctx)
	require.NoError(t, err)
}

// TestBridge_HasStubDaser verifies that a bridge node implements a stub daser that returns an
// error and empty das.SamplingStats
func TestBridge_HasStubDaser(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	_, client := core.StartTestClient(ctx, t)
	nd := TestNode(t, node.Bridge, coremodule.WithClient(client))
	require.NotNil(t, nd)
	err := nd.Start(ctx)
	require.NoError(t, err)

	stats, err := nd.DASer.SamplingStats(ctx)
	assert.EqualError(t, err, "moddas: dasing is not available on bridge nodes")
	assert.Equal(t, stats, das.SamplingStats{})

	err = nd.Stop(ctx)
	require.NoError(t, err)
}
