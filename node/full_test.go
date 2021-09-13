package node

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func TestNewFull(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Core.EmbeddedConfig = core.TestConfig(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.Core.EmbeddedConfig.RootDir)
	})

	nd, err := NewFull(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, nd)
	assert.NotNil(t, nd.Config)
	assert.NotZero(t, nd.Type)
}

func TestFullLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Core.EmbeddedConfig = core.TestConfig(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.Core.EmbeddedConfig.RootDir)
	})

	node, err := NewFull(cfg)
	assert.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotZero(t, node.Type)
	require.NotNil(t, node.CoreClient)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Stop(ctx)
	assert.NoError(t, err)
}
