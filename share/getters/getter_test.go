package getters

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestStoreGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tmpDir := t.TempDir()
	storeCfg := eds.DefaultParameters()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(storeCfg, tmpDir, ds)
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
				share, err := sg.GetShare(ctx, dah, i, j)
				require.NoError(t, err)
				assert.Equal(t, randEds.GetCell(uint(i), uint(j)), share)
			}
		}

		// doesn't panic on indexes too high
		_, err := sg.GetShare(ctx, dah, squareSize, squareSize)
		require.ErrorIs(t, err, share.ErrOutOfBounds)

		// root not found
		_, dah = randomEDS(t)
		_, err = sg.GetShare(ctx, dah, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		randEds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, dah)
		require.NoError(t, err)
		assert.True(t, randEds.Equals(retrievedEDS))

		// root not found
		root := share.Root{}
		_, err = sg.GetEDS(ctx, &root)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		randEds, namespace, dah := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		shares, err := sg.GetSharesByNamespace(ctx, dah, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(dah, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, dah, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// root not found
		root := share.Root{}
		_, err = sg.GetSharesByNamespace(ctx, &root, namespace)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetSharesFromNamespace removes corrupted shard", func(t *testing.T) {
		randEds, namespace, dah := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		// available
		shares, err := sg.GetSharesByNamespace(ctx, dah, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(dah, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// 'corrupt' existing CAR by overwriting with a random EDS
		f, err := os.OpenFile(tmpDir+"/blocks/"+dah.String(), os.O_WRONLY, 0644)
		require.NoError(t, err)
		edsToOverwriteWith, dah := randomEDS(t)
		err = eds.WriteEDS(ctx, edsToOverwriteWith, f)
		require.NoError(t, err)

		shares, err = sg.GetSharesByNamespace(ctx, dah, namespace)
		require.ErrorIs(t, err, share.ErrNotFound)
		require.Nil(t, shares)

		// corruption detected, shard is removed
		has, err := edsStore.Has(ctx, dah.Hash())
		require.False(t, has)
		require.NoError(t, err)
	})
}

func TestIPLDGetter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	storeCfg := eds.DefaultParameters()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	edsStore, err := eds.NewStore(storeCfg, t.TempDir(), ds)
	require.NoError(t, err)

	err = edsStore.Start(ctx)
	require.NoError(t, err)

	bStore := edsStore.Blockstore()
	bserv := ipld.NewBlockservice(bStore, offline.Exchange(edsStore.Blockstore()))
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
				share, err := sg.GetShare(ctx, dah, i, j)
				require.NoError(t, err)
				assert.Equal(t, randEds.GetCell(uint(i), uint(j)), share)
			}
		}

		// doesn't panic on indexes too high
		_, err := sg.GetShare(ctx, dah, squareSize+1, squareSize+1)
		require.ErrorIs(t, err, share.ErrOutOfBounds)

		// root not found
		_, dah = randomEDS(t)
		_, err = sg.GetShare(ctx, dah, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		randEds, dah := randomEDS(t)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, dah)
		require.NoError(t, err)
		assert.True(t, randEds.Equals(retrievedEDS))

		// Ensure blocks still exist after cleanup
		colRoots, _ := retrievedEDS.ColRoots()
		has, err := bStore.Has(ctx, ipld.MustCidFromNamespacedSha256(colRoots[0]))
		assert.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		randEds, namespace, dah := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, dah.Hash(), randEds)
		require.NoError(t, err)

		// first check that shares are returned correctly if they exist
		shares, err := sg.GetSharesByNamespace(ctx, dah, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(dah, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, dah, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// nid doesnt exist in root
		root := share.Root{}
		emptyShares, err = sg.GetSharesByNamespace(ctx, &root, namespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())
	})
}

func randomEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.Root) {
	eds := edstest.RandEDS(t, 4)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	return eds, dah
}

// randomEDSWithDoubledNamespace generates a random EDS and ensures that there are two shares in the
// middle that share a namespace.
func randomEDSWithDoubledNamespace(t *testing.T, size int) (*rsmt2d.ExtendedDataSquare, []byte, *share.Root) {
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

	return eds, share.GetNamespace(randShares[idx1]), dah
}
