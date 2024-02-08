package store

import (
	"context"
	mrand "math/rand"
	"testing"
	"time"

	ds "github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/store/cache"
)

//TODO:
// - add caching tests
// - add recontruction tests

func TestBlockstoreGetShareSample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	edsStore, err := NewStore(DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	// disable cache
	edsStore.cache = cache.NewDoubleCache(cache.NoopCache{}, cache.NoopCache{})

	height := uint64(100)
	eds, dah := randomEDS(t)

	f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	bs := NewBlockstore(edsStore, ds_sync.MutexWrap(ds.NewMapDatastore()))

	t.Run("Sample", func(t *testing.T) {
		width := int(eds.Width())
		for i := 0; i < width*width; i++ {
			id, err := shwap.NewSampleID(height, i, dah)
			require.NoError(t, err)
			blk, err := bs.Get(ctx, id.Cid())
			require.NoError(t, err)

			sample, err := shwap.SampleFromBlock(blk)
			require.NoError(t, err)

			err = sample.Verify(dah)
			require.NoError(t, err)
			require.EqualValues(t, id, sample.SampleID)
		}
	})

	t.Run("Row", func(t *testing.T) {
		width := int(eds.Width())
		for i := 0; i < width; i++ {
			rowID, err := shwap.NewRowID(height, uint16(i), dah)
			require.NoError(t, err)

			blk, err := bs.Get(ctx, rowID.Cid())
			require.NoError(t, err)

			row, err := shwap.RowFromBlock(blk)
			require.NoError(t, err)

			err = row.Verify(dah)
			require.NoError(t, err)

			require.EqualValues(t, rowID, row.RowID)
		}
	})

	t.Run("NamespaceData", func(t *testing.T) {
		size := 8
		namespace := sharetest.RandV0Namespace()
		amount := mrand.Intn(size*size-1) + 1
		eds, dah := edstest.RandEDSWithNamespace(t, namespace, amount, size)

		height := uint64(42)
		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		for i, row := range dah.RowRoots {
			if namespace.IsOutsideRange(row, row) {
				continue
			}

			dataID, err := shwap.NewDataID(height, uint16(i), namespace, dah)
			require.NoError(t, err)

			blk, err := bs.Get(ctx, dataID.Cid())
			require.NoError(t, err)

			nd, err := shwap.DataFromBlock(blk)
			require.NoError(t, err)

			err = nd.Verify(dah)
			require.NoError(t, err)

			require.EqualValues(t, dataID, nd.DataID)
		}
	})
}
