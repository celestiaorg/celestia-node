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

func TestDetectFirstRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("FirstRunArchival", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

		cfg := &Config{EnableService: false}

		err := detectFirstRun(ctx, cfg, ds, 1)
		assert.NoError(t, err)

		nsWrapped := namespace.Wrap(ds, storePrefix)
		prevMode, err := nsWrapped.Get(ctx, previousModeKey)
		require.NoError(t, err)
		assert.Equal(t, []byte("archival"), prevMode)
	})

	t.Run("FirstRunPruned", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

		cfg := &Config{EnableService: true}

		err := detectFirstRun(ctx, cfg, ds, 1)
		assert.NoError(t, err)

		nsWrapped := namespace.Wrap(ds, storePrefix)
		prevMode, err := nsWrapped.Get(ctx, previousModeKey)
		require.NoError(t, err)
		assert.Equal(t, []byte("pruned"), prevMode)
	})

	t.Run("RevertToArchivalNotAllowed", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

		// create archival node instance over a node that has been pruned before
		// (height 500)
		cfg := &Config{EnableService: false}
		lastPrunedHeight := uint64(500)

		err := detectFirstRun(ctx, cfg, ds, lastPrunedHeight)
		assert.Error(t, err)
		assert.ErrorIs(t, err, fullavail.ErrDisallowRevertToArchival)
	})
}
