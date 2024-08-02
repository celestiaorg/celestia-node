package full

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
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

	err = store.Put(ctx, roots, eh.Height(), eds)
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
