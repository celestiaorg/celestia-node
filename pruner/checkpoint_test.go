package pruner

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreCheckpoint(t *testing.T) {
	ctx := context.Background()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	c := &checkpoint{
		PrunerKind:       "test",
		LastPrunedHeight: 1,
		FailedHeaders:    map[uint64]struct{}{1: {}},
	}

	err := storeCheckpoint(ctx, ds, c)
	require.NoError(t, err)

	c2, err := getCheckpoint(ctx, ds)
	require.NoError(t, err)
	require.Equal(t, c, c2)
}

// TestDisallowRevertArchival tests that a node that has been previously run
// with full pruning cannot convert back into an "archival" node.
func TestDisallowRevertArchival(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	namespaceWrapped := namespace.Wrap(ds, storePrefix)
	c := &checkpoint{
		PrunerKind:       "full",
		LastPrunedHeight: 1,
		FailedHeaders:    map[uint64]struct{}{1: {}},
	}

	err := storeCheckpoint(ctx, namespaceWrapped, c)
	require.NoError(t, err)

	// give the unwrapped ds here as this is expected to run
	// before pruner service is constructed
	err = DetectPreviousRun(ctx, ds, "archival")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDisallowRevertToArchival)

	// ensure no false positives
	err = DetectPreviousRun(ctx, ds, "full")
	assert.NoError(t, err)

	// ensure checkpoint is retrievable after
	cp, err := getCheckpoint(ctx, namespaceWrapped)
	require.NoError(t, err)
	require.NotNil(t, cp)
	assert.Equal(t, cp.LastPrunedHeight, c.LastPrunedHeight)
}

func TestCheckpointOverride(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	namespaceWrapped := namespace.Wrap(ds, storePrefix)
	c := &checkpoint{
		PrunerKind:       "archival",
		LastPrunedHeight: 600,
		FailedHeaders:    map[uint64]struct{}{1: {}},
	}

	err := storeCheckpoint(ctx, namespaceWrapped, c)
	require.NoError(t, err)

	// give the unwrapped ds here as this is expected to run
	// before pruner service is constructed
	err = DetectPreviousRun(ctx, ds, "full")
	assert.NoError(t, err)

	cp, err := getCheckpoint(ctx, namespaceWrapped)
	require.NoError(t, err)
	assert.Equal(t, "full", cp.PrunerKind)
	assert.Equal(t, uint64(1), cp.LastPrunedHeight)
}
