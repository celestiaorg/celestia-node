package store

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
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
				idx := shwap.SampleCoords{Row: i, Col: j}

				smpls, err := sg.GetSamples(ctx, eh, []shwap.SampleCoords{idx})
				require.NoError(t, err)
				require.Equal(t, eds.GetCell(uint(i), uint(j)), smpls[0].Share.ToBytes())
			}
		}

		// doesn't panic on indexes too high
		bigIdx := squareSize * squareSize
		_, err = sg.GetSamples(ctx, eh, []shwap.SampleCoords{{Row: bigIdx, Col: bigIdx}})
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

	t.Run("GetRow", func(t *testing.T) {
		eds, roots := randomEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		err := edsStore.PutODSQ4(ctx, eh.DAH, height, eds)
		require.NoError(t, err)

		for i := 0; i < len(eh.DAH.RowRoots); i++ {
			row, err := sg.GetRow(ctx, eh, i)
			require.NoError(t, err)
			retreivedShrs, err := row.Shares()
			require.NoError(t, err)
			edsShrs, err := libshare.FromBytes(eds.Row(uint(i)))
			require.NoError(t, err)
			require.Equal(t, edsShrs, retreivedShrs)
		}
	})

	t.Run("GetNamespaceData", func(t *testing.T) {
		ns := libshare.RandomNamespace()
		eds, roots := edstest.RandEDSWithNamespace(t, ns, 8, 16)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)
		err := edsStore.PutODSQ4(ctx, eh.DAH, height, eds)
		require.NoError(t, err)

		shares, err := sg.GetNamespaceData(ctx, eh, ns)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(eh.DAH, ns))

		// namespace not found
		randNamespace := libshare.RandomNamespace()
		emptyShares, err := sg.GetNamespaceData(ctx, eh, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// root not found
		eh.RawHeader.Height = 666
		_, err = sg.GetNamespaceData(ctx, eh, ns)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})
}
