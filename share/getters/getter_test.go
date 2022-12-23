package getters

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

func TestStoreGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)

	err = edsStore.Start(ctx)
	require.NoError(t, err)

	sg := NewStoreGetter(edsStore)

	t.Run("GetShare", func(t *testing.T) {
		eds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		squareSize := int(eds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, &dah, i, j)
				require.NoError(t, err)
				assert.Equal(t, eds.GetCell(uint(i), uint(j)), share)
			}
		}
	})

	t.Run("GetEDS", func(t *testing.T) {
		eds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, &dah)
		require.NoError(t, err)
		assert.True(t, share.EqualEDS(eds, retrievedEDS))
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		eds, nID, dah := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		shares, err := sg.GetSharesByNamespace(ctx, &dah, nID)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(&dah, nID))
		assert.Len(t, shares.Flatten(), 2)
	})

}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, share.Root) {
	eds := share.RandEDS(t, 4)
	dah := da.NewDataAvailabilityHeader(eds)

	return eds, dah
}

func randomEDSWithDoubledNamespace(t *testing.T, size int) (*rsmt2d.ExtendedDataSquare, []byte, share.Root) {
	n := size * size
	randShares := share.RandShares(t, n)
	idx1 := (n - 1) / 2
	idx2 := n / 2
	// make it so that two rows have the same namespace ID
	copy(randShares[idx2][:8], randShares[idx1][:8])

	eds, err := rsmt2d.ComputeExtendedDataSquare(
		randShares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(size)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	dah := da.NewDataAvailabilityHeader(eds)

	return eds, randShares[idx1][:8], dah
}
