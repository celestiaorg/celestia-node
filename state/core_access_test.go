package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecycle(t *testing.T) {
	ca := NewCoreAccessor(nil, nil, "", "", "")
	ctx, cancel := context.WithCancel(context.Background())
	// start the accessor
	err := ca.Start(ctx)
	require.NoError(t, err)
	// ensure accessor isn't stopped
	require.False(t, ca.IsStopped(ctx))
	// cancel the top level context (this should not affect the lifecycle of the
	// accessor as it should manage its own internal context)
	cancel()
	// ensure accessor was unaffected by top-level context cancellation
	require.False(t, ca.IsStopped(ctx))
	// stop the accessor
	stopCtx, stopCancel := context.WithCancel(context.Background())
	t.Cleanup(stopCancel)
	err = ca.Stop(stopCtx)
	require.NoError(t, err)
	// ensure accessor is stopped
	require.True(t, ca.IsStopped(ctx))
	// ensure that stopping the accessor again does not return an error
	err = ca.Stop(stopCtx)
	require.NoError(t, err)
}
