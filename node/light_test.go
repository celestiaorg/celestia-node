package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLight(t *testing.T) {
	repo := MockRepository(t, DefaultLightConfig())
	nd, err := NewLight(repo)
	require.NoError(t, err)
	require.NotNil(t, nd)
	require.NotNil(t, nd.Config)
	assert.NotZero(t, nd.Type)
}

func TestLightLifecycle(t *testing.T) {
	repo := MockRepository(t, DefaultLightConfig())
	nd, err := NewLight(repo)
	require.NoError(t, err)

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Start(startCtx)
	require.NoError(t, err)

	stopCtx, stopCtxCancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		stopCtxCancel()
	})

	err = nd.Stop(stopCtx)
	require.NoError(t, err)
}
