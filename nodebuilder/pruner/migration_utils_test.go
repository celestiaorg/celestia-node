package pruner

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fullavail "github.com/celestiaorg/celestia-node/share/availability/full"
)

// TestDisallowRevertArchival tests that a node that has been previously run
// with full pruning cannot convert back into an "archival" node
func TestDisallowRevertArchival(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// create a pruned node instance (non-archival) for the first time
	cfg := &Config{EnableService: true}
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	nsWrapped := namespace.Wrap(ds, storePrefix)
	err := nsWrapped.Put(ctx, previousModeKey, pruned)
	require.NoError(t, err)

	convert, err := convertFromArchivalToPruned(ctx, cfg, nsWrapped)
	assert.NoError(t, err)
	assert.False(t, convert)
	// ensure availability impl recorded the pruned run
	prevMode, err := nsWrapped.Get(ctx, previousModeKey)
	require.NoError(t, err)
	assert.Equal(t, pruned, prevMode)

	// now change to archival mode
	cfg.EnableService = false

	// ensure failure
	convert, err = convertFromArchivalToPruned(ctx, cfg, nsWrapped)
	assert.Error(t, err)
	assert.ErrorIs(t, err, fullavail.ErrDisallowRevertToArchival)
	assert.False(t, convert)

	// ensure the node can still run in pruned mode
	cfg.EnableService = true
	convert, err = convertFromArchivalToPruned(ctx, cfg, nsWrapped)
	assert.NoError(t, err)
	assert.False(t, convert)
}

// TestAllowConversionFromArchivalToPruned tests that a node that has been previously run
// in archival mode can convert to a pruned node
func TestAllowConversionFromArchivalToPruned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	nsWrapped := namespace.Wrap(ds, storePrefix)
	err := nsWrapped.Put(ctx, previousModeKey, archival)
	require.NoError(t, err)

	cfg := &Config{EnableService: false}

	convert, err := convertFromArchivalToPruned(ctx, cfg, nsWrapped)
	assert.NoError(t, err)
	assert.False(t, convert)

	cfg.EnableService = true

	convert, err = convertFromArchivalToPruned(ctx, cfg, nsWrapped)
	assert.NoError(t, err)
	assert.True(t, convert)

	prevMode, err := nsWrapped.Get(ctx, previousModeKey)
	require.NoError(t, err)
	assert.Equal(t, pruned, prevMode)
}

func TestDetectFirstRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("FirstRunArchival", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
		nsWrapped := namespace.Wrap(ds, storePrefix)

		cfg := &Config{EnableService: false}

		err := detectFirstRun(ctx, cfg, nsWrapped, 1)
		assert.NoError(t, err)

		prevMode, err := nsWrapped.Get(ctx, previousModeKey)
		require.NoError(t, err)
		assert.Equal(t, archival, prevMode)
	})

	t.Run("FirstRunPruned", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
		nsWrapped := namespace.Wrap(ds, storePrefix)

		cfg := &Config{EnableService: true}

		err := detectFirstRun(ctx, cfg, nsWrapped, 1)
		assert.NoError(t, err)

		prevMode, err := nsWrapped.Get(ctx, previousModeKey)
		require.NoError(t, err)
		assert.Equal(t, pruned, prevMode)
	})

	t.Run("RevertToArchivalNotAllowed", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
		nsWrapped := namespace.Wrap(ds, storePrefix)

		cfg := &Config{EnableService: false}

		err := detectFirstRun(ctx, cfg, nsWrapped, 500)
		assert.Error(t, err)
		assert.ErrorIs(t, err, fullavail.ErrDisallowRevertToArchival)
	})
}
