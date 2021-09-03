package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLight(t *testing.T) {
	nd, err := NewLight(DefaultConfig())
	assert.NoError(t, err)
	assert.NotNil(t, nd)
	assert.NotNil(t, nd.Config)
	assert.NotZero(t, nd.Type)
}

func TestLightLifecycle(t *testing.T) {
	nd, err := NewLight(DefaultConfig())
	require.NoError(t, err)

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Start(startCtx)
	require.NoError(t, err)

	stopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Stop(stopCtx)
	require.NoError(t, err)
}
