package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFull(t *testing.T) {
	nd, err := NewFull(DefaultConfig())
	assert.NoError(t, err)
	assert.NotNil(t, nd)
	assert.NotNil(t, nd.Config)
	assert.NotZero(t, nd.Type)
}

func TestFullLifecycle(t *testing.T) {
	nd, err := NewFull(DefaultConfig())
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
