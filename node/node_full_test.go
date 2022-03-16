package node

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFull(t *testing.T) {
	store := MockStore(t, DefaultConfig(Full))
	nd, err := New(Full, store)
	require.NoError(t, err)
	require.NotNil(t, nd)
	require.NotNil(t, nd.Config)
	require.NotNil(t, nd.HeaderServ)
	assert.NotZero(t, nd.Type)
}

func TestFullLifecycle(t *testing.T) {
	store := MockStore(t, DefaultConfig(Full))
	nd, err := New(Full, store)
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
