package full

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
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
	avail := NewShareAvailability(store, getter)
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
	avail := NewShareAvailability(store, nil)

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
	avail := NewShareAvailability(store, getter)

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

	avail := NewShareAvailability(store, getter)
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

	avail := NewShareAvailability(store, getter, WithArchivalMode())
	avail.storageWindow = time.Nanosecond // make all headers outside sampling window

	err = avail.SharesAvailable(ctx, eh)
	require.NoError(t, err)
	has, err := store.HasByHash(ctx, roots.Hash())
	require.NoError(t, err)
	assert.True(t, has)
}
