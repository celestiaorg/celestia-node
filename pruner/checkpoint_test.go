package pruner

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/pruner/archival"
	fullavail "github.com/celestiaorg/celestia-node/share/availability/full"
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

func TestCheckpoint_ArchivalToPruned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	getter := headertest.NewStore(t)

	arch := archival.NewPruner(store)

	serv, err := NewService(arch, fullavail.DisableStorageWindow, getter, ds, time.Second)
	require.NoError(t, err)

	err = serv.loadCheckpoint(ctx)
	// TODO @renaynay: finish!!!!!!!
}
