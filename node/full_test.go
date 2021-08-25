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

	stop, err := NewFull(startCtx, &config.Config{})
	assert.NoError(t, err)
	require.NotNil(t, stop)

	stopCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = stop(stopCtx)
	assert.NoError(t, err)
}
