//go:build test_unit

package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFullAndLifecycle(t *testing.T) {
	node := TestNode(t, Full)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotNil(t, node.HeaderServ)
	assert.NotZero(t, node.Type)

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := node.Start(startCtx)
	require.NoError(t, err)

	stopCtx, stopCtxCancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		stopCtxCancel()
	})

	err = node.Stop(stopCtx)
	require.NoError(t, err)
}
