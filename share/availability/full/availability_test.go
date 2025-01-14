package full

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
	"github.com/celestiaorg/celestia-node/store"
)

func TestSharesAvailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// RandServiceWithSquare creates a NewShareAvailability inside, so we can test it
	eds := edstest.RandEDS(t, 16)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)

	getter := mock.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().GetEDS(gomock.Any(), eh).Return(eds, nil)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	avail := NewShareAvailability(store, getter, datastore.NewMapDatastore())
	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	// Check if the store has the root
	has, err := store.HasByHash(ctx, roots.Hash())
	require.NoError(t, err)
	require.True(t, has)

	// Check if the store has the root linked to the height
	has, err = store.HasByHeight(ctx, eh.Height())
	require.NoError(t, err)
	require.True(t, has)
}

func TestSharesAvailable_StoredEds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)
	require.NoError(t, err)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	avail := NewShareAvailability(store, nil, datastore.NewMapDatastore())

	err = store.PutODSQ4(ctx, roots, eh.Height(), eds)
	require.NoError(t, err)

	has, err := store.HasByHeight(ctx, eh.Height())
	require.NoError(t, err)
	require.True(t, has)

	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)

	has, err = store.HasByHeight(ctx, eh.Height())
	require.NoError(t, err)
	require.True(t, has)
}

func TestSharesAvailable_ErrNotAvailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	getter := mock.NewMockGetter(ctrl)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)
	require.NoError(t, err)

	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	avail := NewShareAvailability(store, getter, datastore.NewMapDatastore())

	errors := []error{shwap.ErrNotFound, context.DeadlineExceeded}
	for _, getterErr := range errors {
		getter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).Return(nil, getterErr)
		err := avail.SharesAvailable(ctx, eh)
		require.ErrorIs(t, err, share.ErrNotAvailable)
	}
}

// TestSharesAvailable_OutsideSamplingWindow_NonArchival tests to make sure
// blocks are skipped that are outside sampling window.
func TestSharesAvailable_OutsideSamplingWindow_NonArchival(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	getter := mock.NewMockGetter(gomock.NewController(t))
	getter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).Times(0)
	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	suite := headertest.NewTestSuite(t, 3, time.Nanosecond)
	headers := suite.GenExtendedHeaders(10)

	avail := NewShareAvailability(store, getter, datastore.NewMapDatastore())
	avail.storageWindow = time.Nanosecond // make all headers outside sampling window

	for _, h := range headers {
		err := avail.SharesAvailable(ctx, h)
		assert.ErrorIs(t, err, availability.ErrOutsideSamplingWindow)
	}
}

// TestSharesAvailable_OutsideSamplingWindow_Archival tests to make sure
// blocks are still synced that are outside sampling window.
func TestSharesAvailable_OutsideSamplingWindow_Archival(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	getter := mock.NewMockGetter(gomock.NewController(t))
	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	eh := headertest.RandExtendedHeaderWithRoot(t, roots)
	require.NoError(t, err)

	getter.EXPECT().GetEDS(gomock.Any(), gomock.Any()).Times(1).Return(eds, nil)

	avail := NewShareAvailability(store, getter, datastore.NewMapDatastore(), WithArchivalMode())
	avail.storageWindow = time.Nanosecond // make all headers outside sampling window

	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
	has, err := store.HasByHash(ctx, roots.Hash())
	require.NoError(t, err)
	assert.True(t, has)
}

// TestDisallowRevertArchival tests that a node that has been previously run
// with full pruning cannot convert back into an "archival" node
func TestDisallowRevertArchival(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	nsWrapped := namespace.Wrap(ds, storePrefix)
	err := nsWrapped.Put(ctx, previousModeKey, pruned)
	require.NoError(t, err)

	// create a pruned node instance (non-archival) for the first time
	fa := NewShareAvailability(nil, nil, ds)

	convert, err := fa.ConvertFromArchivalToPruned(ctx)
	assert.NoError(t, err)
	assert.False(t, convert)
	// ensure availability impl recorded the pruned run
	prevMode, err := fa.ds.Get(ctx, previousModeKey)
	require.NoError(t, err)
	assert.Equal(t, pruned, prevMode)

	// now change to archival mode
	fa = NewShareAvailability(nil, nil, ds, WithArchivalMode())

	// ensure failure
	convert, err = fa.ConvertFromArchivalToPruned(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrDisallowRevertToArchival)
	assert.False(t, convert)

	// ensure the node can still run in pruned mode
	fa = NewShareAvailability(nil, nil, ds)
	convert, err = fa.ConvertFromArchivalToPruned(ctx)
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

	fa := NewShareAvailability(nil, nil, ds, WithArchivalMode())

	convert, err := fa.ConvertFromArchivalToPruned(ctx)
	assert.NoError(t, err)
	assert.False(t, convert)

	fa = NewShareAvailability(nil, nil, ds)

	convert, err = fa.ConvertFromArchivalToPruned(ctx)
	assert.NoError(t, err)
	assert.True(t, convert)

	prevMode, err := fa.ds.Get(ctx, previousModeKey)
	require.NoError(t, err)
	assert.Equal(t, pruned, prevMode)
}

func TestDetectFirstRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("FirstRunArchival", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

		fa := NewShareAvailability(nil, nil, ds, WithArchivalMode())
		err := DetectFirstRun(ctx, fa, 1)
		assert.NoError(t, err)

		prevMode, err := fa.ds.Get(ctx, previousModeKey)
		require.NoError(t, err)
		assert.Equal(t, archival, prevMode)
	})

	t.Run("FirstRunPruned", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

		fa := NewShareAvailability(nil, nil, ds)
		err := DetectFirstRun(ctx, fa, 1)
		assert.NoError(t, err)

		prevMode, err := fa.ds.Get(ctx, previousModeKey)
		require.NoError(t, err)
		assert.Equal(t, pruned, prevMode)
	})

	t.Run("RevertToArchivalNotAllowed", func(t *testing.T) {
		ds := ds_sync.MutexWrap(datastore.NewMapDatastore())

		fa := NewShareAvailability(nil, nil, ds, WithArchivalMode())
		err := DetectFirstRun(ctx, fa, 500)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrDisallowRevertToArchival)
	})
}
