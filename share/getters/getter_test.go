package getters

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	dsbadger "github.com/ipfs/go-ds-badger4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
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
		randEds, eh := randomEDS(t)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

		squareSize := int(randEds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, eh, i, j)
				require.NoError(t, err)
				assert.Equal(t, randEds.GetCell(uint(i), uint(j)), share)
			}
		}

		// doesn't panic on indexes too high
		_, err := sg.GetShare(ctx, eh, squareSize, squareSize)
		require.ErrorIs(t, err, share.ErrOutOfBounds)

		// root not found
		_, eh = randomEDS(t)
		_, err = sg.GetShare(ctx, eh, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		randEds, eh := randomEDS(t)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, eh)
		require.NoError(t, err)
		assert.True(t, randEds.Equals(retrievedEDS))

		// root not found
		emptyRoot := da.MinDataAvailabilityHeader()
		eh.DAH = &emptyRoot
		_, err = sg.GetEDS(ctx, eh)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		randEds, namespace, eh := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

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
		emptyRoot := da.MinDataAvailabilityHeader()
		eh.DAH = &emptyRoot
		_, err = sg.GetSharesByNamespace(ctx, eh, namespace)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetSharesFromNamespace removes corrupted shard", func(t *testing.T) {
		randEds, namespace, eh := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

		// available
		shares, err := sg.GetSharesByNamespace(ctx, eh, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(eh.DAH, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// 'corrupt' existing CAR by overwriting with a random EDS
		f, err := os.OpenFile(tmpDir+"/blocks/"+eh.DAH.String(), os.O_WRONLY, 0o644)
		require.NoError(t, err)
		edsToOverwriteWith, eh := randomEDS(t)
		err = eds.WriteEDS(ctx, edsToOverwriteWith, f)
		require.NoError(t, err)

		shares, err = sg.GetSharesByNamespace(ctx, eh, namespace)
		require.ErrorIs(t, err, share.ErrNotFound)
		require.Nil(t, shares)

		// corruption detected, shard is removed
		// try every 200ms until it passes or the context ends
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.Fatal("context ended before successful retrieval")
			case <-ticker.C:
				has, err := edsStore.Has(ctx, eh.DAH.Hash())
				if err != nil {
					t.Fatal(err)
				}
				if !has {
					require.NoError(t, err)
					return
				}
			}
		}
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

		randEds, eh := randomEDS(t)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

		squareSize := int(randEds.Width())
		for i := 0; i < squareSize; i++ {
			for j := 0; j < squareSize; j++ {
				share, err := sg.GetShare(ctx, eh, i, j)
				require.NoError(t, err)
				assert.Equal(t, randEds.GetCell(uint(i), uint(j)), share)
			}
		}

		// doesn't panic on indexes too high
		_, err := sg.GetShare(ctx, eh, squareSize+1, squareSize+1)
		require.ErrorIs(t, err, share.ErrOutOfBounds)

		// root not found
		_, eh = randomEDS(t)
		_, err = sg.GetShare(ctx, eh, 0, 0)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("GetEDS", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		randEds, eh := randomEDS(t)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

		retrievedEDS, err := sg.GetEDS(ctx, eh)
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

		randEds, namespace, eh := randomEDSWithDoubledNamespace(t, 4)
		err = edsStore.Put(ctx, eh.DAH.Hash(), randEds)
		require.NoError(t, err)

		// first check that shares are returned correctly if they exist
		shares, err := sg.GetSharesByNamespace(ctx, eh, namespace)
		require.NoError(t, err)
		require.NoError(t, shares.Verify(eh.DAH, namespace))
		assert.Len(t, shares.Flatten(), 2)

		// namespace not found
		randNamespace := sharetest.RandV0Namespace()
		emptyShares, err := sg.GetSharesByNamespace(ctx, eh, randNamespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())

		// nid doesn't exist in root
		emptyRoot := da.MinDataAvailabilityHeader()
		eh.DAH = &emptyRoot
		emptyShares, err = sg.GetSharesByNamespace(ctx, eh, namespace)
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())
	})
}

// BenchmarkIPLDGetterOverBusyCache benchmarks the performance of the IPLDGetter when the
// cache size of the underlying blockstore is less than the number of blocks being requested in
// parallel. This is to ensure performance doesn't degrade when the cache is being frequently
// evicted.
// BenchmarkIPLDGetterOverBusyCache-10/128    	       1	12460428417 ns/op (~12s)
func BenchmarkIPLDGetterOverBusyCache(b *testing.B) {
	const (
		blocks = 10
		size   = 128
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	b.Cleanup(cancel)

	dir := b.TempDir()
	ds, err := dsbadger.NewDatastore(dir, &dsbadger.DefaultOptions)
	require.NoError(b, err)

	newStore := func(params *eds.Parameters) *eds.Store {
		edsStore, err := eds.NewStore(params, dir, ds)
		require.NoError(b, err)
		err = edsStore.Start(ctx)
		require.NoError(b, err)
		return edsStore
	}
	edsStore := newStore(eds.DefaultParameters())

	// generate EDSs and store them
	headers := make([]*header.ExtendedHeader, blocks)
	for i := range headers {
		eds := edstest.RandEDS(b, size)
		dah, err := da.NewDataAvailabilityHeader(eds)
		require.NoError(b, err)
		err = edsStore.Put(ctx, dah.Hash(), eds)
		require.NoError(b, err)

		eh := headertest.RandExtendedHeader(b)
		eh.DAH = &dah

		// store cids for read loop later
		headers[i] = eh
	}

	// restart store to clear cache
	require.NoError(b, edsStore.Stop(ctx))

	// set BlockstoreCacheSize to 1 to force eviction on every read
	params := eds.DefaultParameters()
	params.BlockstoreCacheSize = 1
	edsStore = newStore(params)
	bstore := edsStore.Blockstore()
	bserv := ipld.NewBlockservice(bstore, offline.Exchange(bstore))

	// start client
	getter := NewIPLDGetter(bserv)

	// request blocks in parallel
	b.ResetTimer()
	g := sync.WaitGroup{}
	g.Add(blocks)
	for _, h := range headers {
		h := h
		go func() {
			defer g.Done()
			_, err := getter.GetEDS(ctx, h)
			require.NoError(b, err)
		}()
	}
	g.Wait()
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
