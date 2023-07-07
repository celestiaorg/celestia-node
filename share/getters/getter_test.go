package getters

import (
	"context"
	"testing"
	"time"

	bsrv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestTeeGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)

	err = edsStore.Start(ctx)
	require.NoError(t, err)

	bServ := mdutils.Bserv()
	ig := NewIPLDGetter(bServ)
	tg := NewTeeGetter(ig, edsStore)

	t.Run("TeesToEDSStore", func(t *testing.T) {
		randEds, dah := randomEDS(t)
		_, err := ipld.ImportShares(ctx, randEds.Flattened(), bServ)
		require.NoError(t, err)

		// eds store doesn't have the EDS yet
		ok, err := edsStore.Has(ctx, dah.Hash())
		assert.False(t, ok)
		assert.NoError(t, err)

		retrievedEDS, err := tg.GetEDS(ctx, &dah)
		require.NoError(t, err)
		require.True(t, share.EqualEDS(randEds, retrievedEDS))

		// eds store now has the EDS and it can be retrieved
		ok, err = edsStore.Has(ctx, dah.Hash())
		assert.True(t, ok)
		assert.NoError(t, err)
		finalEDS, err := edsStore.Get(ctx, dah.Hash())
		assert.NoError(t, err)
		require.True(t, share.EqualEDS(randEds, finalEDS))
	})

	t.Run("ShardAlreadyExistsDoesntError", func(t *testing.T) {
		randEds, dah := randomEDS(t)
		_, err := ipld.ImportShares(ctx, randEds.Flattened(), bServ)
		require.NoError(t, err)

		retrievedEDS, err := tg.GetEDS(ctx, &dah)
		require.NoError(t, err)
		require.True(t, share.EqualEDS(randEds, retrievedEDS))

		// no error should be returned, even though the EDS identified by the DAH already exists
		retrievedEDS, err = tg.GetEDS(ctx, &dah)
		require.NoError(t, err)
		require.True(t, share.EqualEDS(randEds, retrievedEDS))
	})
}

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
		randEds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		squareSize := int(randEds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, &dah, i, j)
				require.NoError(t, err)
				assert.Equal(t, randEds.GetCell(uint(i), uint(j)), share)
			}
		}

		// root not found
		_, dah = randomEDS(t)
		_, err = sg.GetShare(ctx, &dah, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		randEds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, &dah)
		require.NoError(t, err)
		assert.True(t, share.EqualEDS(randEds, retrievedEDS))

		// root not found
		root := share.Root{}
		_, err = sg.GetEDS(ctx, &root)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		randEds, namespace, dah := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		shares, err := sg.GetSharesByNamespace(ctx, &dah, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(&dah, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, &dah, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// root not found
		root := share.Root{}
		_, err = sg.GetSharesByNamespace(ctx, &root, namespace)
		require.ErrorIs(t, err, share.ErrNotFound)
	})
}

func TestIPLDGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)

	err = edsStore.Start(ctx)
	require.NoError(t, err)

	bserv := bsrv.New(edsStore.Blockstore(), offline.Exchange(edsStore.Blockstore()))
	sg := NewIPLDGetter(bserv)

	t.Run("GetShare", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		randEds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		squareSize := int(randEds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, &dah, i, j)
				require.NoError(t, err)
				assert.Equal(t, randEds.GetCell(uint(i), uint(j)), share)
			}
		}

		// root not found
		_, dah = randomEDS(t)
		_, err = sg.GetShare(ctx, &dah, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		randEds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, &dah)
		require.NoError(t, err)
		assert.True(t, share.EqualEDS(randEds, retrievedEDS))
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		randEds, namespace, dah := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		// first check that shares are returned correctly if they exist
		shares, err := sg.GetSharesByNamespace(ctx, &dah, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(&dah, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, &dah, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// nid doesnt exist in root
		root := share.Root{}
		emptyShares, err = sg.GetSharesByNamespace(ctx, &root, namespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())
	})
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, share.Root) {
	eds := edstest.RandEDS(t, 4)
	dah := da.NewDataAvailabilityHeader(eds)
	return eds, dah
}

// randomEDSWithDoubledNamespace generates a random EDS and ensures that there are two shares in the
// middle that share a namespace.
func randomEDSWithDoubledNamespace(t *testing.T, size int) (*rsmt2d.ExtendedDataSquare, []byte, share.Root) {
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
	dah := da.NewDataAvailabilityHeader(eds)

	return eds, share.GetNamespace(randShares[idx1]), dah
}
