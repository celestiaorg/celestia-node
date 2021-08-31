package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/config"
)

func TestNewLight(t *testing.T) {
	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nd, err := NewLight(startCtx, &config.Config{})
	assert.NoError(t, err)
	require.NotNil(t, nd)
	require.NotNil(t, nd)
	require.NotNil(t, nd.Config)
	require.NotZero(t, nd.Type)

	stopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Stop(stopCtx)
	assert.NoError(t, err)
}
