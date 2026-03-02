package full

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDisallowRevertArchival tests that a node that has been previously run
// with full pruning cannot convert back into an "archival" node
func TestDisallowRevertArchival(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

	// create a pruned node instance (non-archival) for the first time
	nsWrapped := namespace.Wrap(ds, storePrefix)
	err := nsWrapped.Put(ctx, previousModeKey, pruned)
	require.NoError(t, err)

	convert, err := ConvertFromArchivalToPruned(ctx, ds, false)
	assert.NoError(t, err)
	assert.False(t, convert)
	// ensure availability impl recorded the pruned run
	prevMode, err := nsWrapped.Get(ctx, previousModeKey)
	require.NoError(t, err)
	assert.Equal(t, pruned, prevMode)

	// now change to archival mode, ensure failure
	convert, err = ConvertFromArchivalToPruned(ctx, ds, true)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDisallowRevertToArchival)
	assert.False(t, convert)

	// ensure the node can still run in pruned mode
	convert, err = ConvertFromArchivalToPruned(ctx, ds, false)
	assert.NoError(t, err)
	assert.False(t, convert)
}

// TestAllowConversionFromArchivalToPruned tests that a node that has been previously run
// in archival mode can convert to a pruned node
func TestAllowConversionFromArchivalToPruned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

	// make an archival node
	nsWrapped := namespace.Wrap(ds, storePrefix)
	err := nsWrapped.Put(ctx, previousModeKey, archival)
	require.NoError(t, err)

	convert, err := ConvertFromArchivalToPruned(ctx, ds, true)
	assert.NoError(t, err)
	assert.False(t, convert)

	// turn into a pruned node
	convert, err = ConvertFromArchivalToPruned(ctx, ds, false)
	assert.NoError(t, err)
	assert.True(t, convert)

	prevMode, err := nsWrapped.Get(ctx, previousModeKey)
	require.NoError(t, err)
	assert.Equal(t, pruned, prevMode)
}
