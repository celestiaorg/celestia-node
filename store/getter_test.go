package store

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestStoreGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	edsStore, err := NewStore(DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	sg := NewGetter(edsStore)
	height := atomic.Uint64{}

	t.Run("GetShare", func(t *testing.T) {
		eds, roots := randomEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		err := edsStore.PutODSQ4(ctx, eh.DAH, height, eds)
		require.NoError(t, err)

		squareSize := int(eds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, eh, i, j)
				require.NoError(t, err)
				require.Equal(t, eds.GetCell(uint(i), uint(j)), share)
			}
		}

		// doesn't panic on indexes too high
		_, err = sg.GetShare(ctx, eh, squareSize, squareSize)
		require.ErrorIs(t, err, shwap.ErrOutOfBounds)
	})

	t.Run("GetEDS", func(t *testing.T) {
		eds, roots := randomEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		err := edsStore.PutODSQ4(ctx, eh.DAH, height, eds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, eh)
		require.NoError(t, err)
		require.True(t, eds.Equals(retrievedEDS))

		// root not found
		eh.RawHeader.Height = 666
		_, err = sg.GetEDS(ctx, eh)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		ns := sharetest.RandV0Namespace()
		eds, roots := edstest.RandEDSWithNamespace(t, ns, 8, 16)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)
		err := edsStore.PutODSQ4(ctx, eh.DAH, height, eds)
		require.NoError(t, err)

		shares, err := sg.GetSharesByNamespace(ctx, eh, ns)
		require.NoError(t, err)
		require.NoError(t, shares.Validate(eh.DAH, ns))

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, eh, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// root not found
		eh.RawHeader.Height = 666
		_, err = sg.GetSharesByNamespace(ctx, eh, ns)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})
}
