package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/config"
)

func TestNewFull(t *testing.T) {
	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nd, err := NewFull(startCtx, &config.Config{})
	assert.NoError(t, err)
	require.NotNil(t, nd)

	stopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = nd.Start(stopCtx)
	assert.NoError(t, err)
}
