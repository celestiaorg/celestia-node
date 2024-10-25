package pruner

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/pruner/archival"
	"github.com/celestiaorg/celestia-node/pruner/full"
	"github.com/celestiaorg/celestia-node/store"
)

func TestStoreCheckpoint(t *testing.T) {
	ctx := context.Background()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	c := &checkpoint{
		LastPrunedHeight: 1,
		FailedHeaders:    map[uint64]struct{}{1: {}},
	}

	err := storeCheckpoint(ctx, ds, c)
	require.NoError(t, err)

	c2, err := getCheckpoint(ctx, ds)
	require.NoError(t, err)
	require.Equal(t, c, c2)
}

// TestCheckpoint_ArchivalToPruned tests that a pruner service cannot be switched
// from a full pruner instance to an archival one.
func TestCheckpoint_ArchivalToPruned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	suite := headertest.NewTestSuite(t, 1, time.Millisecond)
	getter := headertest.NewCustomStore(t, suite, 100)

	archPruner := archival.NewPruner(edsStore)
	fullPruner := full.NewPruner(edsStore)

	// start off with archival pruner service
	serv, err := NewService(archPruner, time.Millisecond*5, getter, ds, time.Millisecond)
	require.NoError(t, err)
	serv.ctx, serv.cancel = ctx, cancel

	// ensure checkpoint is initialized correctly
	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, serv.checkpoint.PrunerType, archPruner.Kind())

	// and prune some blocks (this will also update the checkpoint on disk)
	lastPruned, err := serv.lastPruned(ctx)
	require.NoError(t, err)
	lastPruned = serv.prune(ctx, lastPruned)
	assert.Greater(t, lastPruned.Height(), uint64(1))

	// reset pruner to full pruner
	serv, err = NewService(fullPruner, time.Millisecond*5, getter, ds, time.Millisecond)
	require.NoError(t, err)
	serv.ctx, serv.cancel = ctx, cancel

	// ensure checkpoint was reset properly
	err = serv.loadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, serv.checkpoint.PrunerType, fullPruner.Kind())
	assert.Equal(t, uint64(1), serv.checkpoint.LastPrunedHeight)
	// store the checkpoint
	err = serv.updateCheckpoint(ctx, serv.checkpoint.LastPrunedHeight, nil)
	require.NoError(t, err)

	// switch back to archival pruner
	serv, err = NewService(archPruner, time.Millisecond*5, getter, ds, time.Millisecond)
	require.NoError(t, err)
	err = serv.loadCheckpoint(ctx)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "mismatched pruner type provided")
}
