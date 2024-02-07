package getters

import (
	"context"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestStoreGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	sg := NewStoreGetter(edsStore)
	height := atomic.Uint64{}

	t.Run("GetShare", func(t *testing.T) {
		eds, eh := randomEDS(t)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)
		f, err := edsStore.Put(ctx, eh.DAH.Hash(), height, eds)
		require.NoError(t, err)
		defer f.Close()

		squareSize := int(eds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, eh, i, j)
				require.NoError(t, err)
				assert.Equal(t, eds.GetCell(uint(i), uint(j)), share)
			}
		}

		// doesn't panic on indexes too high
		_, err = sg.GetShare(ctx, eh, squareSize, squareSize)
		require.ErrorIs(t, err, share.ErrOutOfBounds)

		// root not found
		_, eh = randomEDS(t)
		_, err = sg.GetShare(ctx, eh, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		eds, eh := randomEDS(t)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)
		f, err := edsStore.Put(ctx, eh.DAH.Hash(), height, eds)
		require.NoError(t, err)
		defer f.Close()

		retrievedEDS, err := sg.GetEDS(ctx, eh)
		require.NoError(t, err)
		assert.True(t, eds.Equals(retrievedEDS))

		// root not found
		eh.RawHeader.Height = 666
		_, err = sg.GetEDS(ctx, eh)
		require.ErrorIs(t, err, share.ErrNotFound, err)
	})

	t.Run("Get empty EDS", func(t *testing.T) {
		// empty root
		emptyRoot := da.MinDataAvailabilityHeader()
		eh := headertest.RandExtendedHeaderWithRoot(t, &emptyRoot)
		f, err := edsStore.Put(ctx, eh.DAH.Hash(), eh.Height(), nil)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		eds, err := sg.GetEDS(ctx, eh)
		require.NoError(t, err)
		dah, err := share.NewRoot(eds)
		require.True(t, share.DataHash(dah.Hash()).IsEmptyRoot())
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		eds, namespace, eh := randomEDSWithDoubledNamespace(t, 4)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)
		f, err := edsStore.Put(ctx, eh.DAH.Hash(), height, eds)
		require.NoError(t, err)
		defer f.Close()

		shares, err := sg.GetSharesByNamespace(ctx, eh, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(eh.DAH, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, eh, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// root not found
		eh.RawHeader.Height = 666
		_, err = sg.GetSharesByNamespace(ctx, eh, namespace)
		require.ErrorIs(t, err, share.ErrNotFound, err)
	})
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *header.ExtendedHeader) {
	eds := edstest.RandEDS(t, 4)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, dah)
	return eds, eh
}

// randomEDSWithDoubledNamespace generates a random EDS and ensures that there are two shares in the
// middle that share a namespace.
func randomEDSWithDoubledNamespace(
	t *testing.T,
	size int,
) (*rsmt2d.ExtendedDataSquare, []byte, *header.ExtendedHeader) {
	n := size * size
	randShares := sharetest.RandShares(t, n)
	idx1 := (n - 1) / 2
	idx2 := n / 2

	// Make it so that the two shares in two different rows have a common
	// namespace. For example if size=4, the original data square looks like
	// this:
	// _ _ _ _
	// _ _ _ D
	// D _ _ _
	// _ _ _ _
	// where the D shares have a common namespace.
	copy(share.GetNamespace(randShares[idx2]), share.GetNamespace(randShares[idx1]))

	eds, err := rsmt2d.ComputeExtendedDataSquare(
		randShares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(size)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	eh := headertest.RandExtendedHeaderWithRoot(t, dah)

	return eds, share.GetNamespace(randShares[idx1]), eh
}
