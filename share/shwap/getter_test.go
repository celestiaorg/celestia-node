package shwap_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/store"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestGetter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	size := 8
	ns := sharetest.RandV0Namespace()
	square, root := edstest.RandEDSWithNamespace(t, ns, size*size, size)
	hdr := &header.ExtendedHeader{RawHeader: header.RawHeader{Height: 1}, DAH: root}

	bstore := edsBlockstore(ctx, t, square, hdr.Height())
	exch := DummySessionExchange{bstore}
	get := shwap.NewGetter(exch, blockstore.NewBlockstore(datastore.NewMapDatastore()))

	t.Run("GetShares", func(t *testing.T) {
		idxs := rand.Perm(int(square.Width() ^ 2))[:10]
		shrs, err := get.GetShares(ctx, hdr, idxs...)
		assert.NoError(t, err)

		for i, shrs := range shrs {
			idx := idxs[i]
			x, y := uint(idx)/square.Width(), uint(idx)%square.Width()
			cell := square.GetCell(x, y)
			ok := bytes.Equal(cell, shrs)
			require.True(t, ok)
		}
	})

	t.Run("GetShares from empty", func(t *testing.T) {
		emptyRoot := da.MinDataAvailabilityHeader()
		eh := headertest.RandExtendedHeaderWithRoot(t, &emptyRoot)

		idxs := []int{0, 1, 2, 3}
		square := share.EmptyExtendedDataSquare()
		shrs, err := get.GetShares(ctx, eh, idxs...)
		assert.NoError(t, err)

		for i, shrs := range shrs {
			idx := idxs[i]
			x, y := uint(idx)/square.Width(), uint(idx)%square.Width()
			cell := square.GetCell(x, y)
			ok := bytes.Equal(cell, shrs)
			require.True(t, ok)
		}
	})

	t.Run("GetEDS", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds, err := get.GetEDS(ctx, hdr)
		assert.NoError(t, err)
		assert.NotNil(t, eds)

		ok := eds.Equals(square)
		assert.True(t, ok)
	})

	t.Run("GetEDS empty", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		emptyRoot := da.MinDataAvailabilityHeader()
		eh := headertest.RandExtendedHeaderWithRoot(t, &emptyRoot)

		eds, err := get.GetEDS(ctx, eh)
		assert.NoError(t, err)
		assert.NotNil(t, eds)

		dah, err := share.NewRoot(eds)
		require.True(t, share.DataHash(dah.Hash()).IsEmptyRoot())
	})

	t.Run("GetSharesByNamespace", func(t *testing.T) {
		nshrs, err := get.GetSharesByNamespace(ctx, hdr, ns)
		assert.NoError(t, err)
		assert.NoError(t, nshrs.Verify(root, ns))
		assert.NotEmpty(t, nshrs.Flatten())

		t.Run("NamespaceOutsideOfRoot", func(t *testing.T) {
			randNamespace := sharetest.RandV0Namespace()
			emptyShares, err := get.GetSharesByNamespace(ctx, hdr, randNamespace)
			assert.NoError(t, err)
			assert.Empty(t, emptyShares)
			assert.NoError(t, emptyShares.Verify(root, randNamespace))
			assert.Empty(t, emptyShares.Flatten())
		})

		t.Run("NamespaceInsideOfRoot", func(t *testing.T) {
			// this test requires a different setup, so we generate a new EDS
			square := edstest.RandEDS(t, 8)
			root, err := share.NewRoot(square)
			require.NoError(t, err)
			hdr := &header.ExtendedHeader{RawHeader: header.RawHeader{Height: 3}, DAH: root}

			bstore := edsBlockstore(ctx, t, square, hdr.Height())
			exch := &DummySessionExchange{bstore}
			get := shwap.NewGetter(exch, blockstore.NewBlockstore(datastore.NewMapDatastore()))

			maxNs := nmt.MaxNamespace(root.RowRoots[(len(root.RowRoots))/2-1], share.NamespaceSize)
			ns, err := share.Namespace(maxNs).AddInt(-1)
			require.NoError(t, err)
			require.Len(t, ipld.FilterRootByNamespace(root, ns), 1)

			emptyShares, err := get.GetSharesByNamespace(ctx, hdr, ns)
			assert.NoError(t, err)
			assert.NotNil(t, emptyShares[0].Proof)
			assert.NoError(t, emptyShares.Verify(root, ns))
			assert.Empty(t, emptyShares.Flatten())
		})
	})
}

type DummySessionExchange struct {
	blockstore.Blockstore
}

func (e DummySessionExchange) NewSession(context.Context) exchange.Fetcher {
	return e
}

func (e DummySessionExchange) GetBlock(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	blk, err := e.Get(ctx, k)
	if format.IsNotFound(err) {
		return nil, fmt.Errorf("block was not found locally (offline): %w", err)
	}
	rbcid, err := k.Prefix().Sum(blk.RawData())
	if err != nil {
		return nil, err
	}

	if !rbcid.Equals(k) {
		return nil, blockstore.ErrHashMismatch
	}
	return blk, err
}

func (e DummySessionExchange) NotifyNewBlocks(context.Context, ...blocks.Block) error {
	return nil
}

func (e DummySessionExchange) GetBlocks(ctx context.Context, ks []cid.Cid) (<-chan blocks.Block, error) {
	out := make(chan blocks.Block)
	go func() {
		defer close(out)
		for _, k := range ks {
			hit, err := e.GetBlock(ctx, k)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case out <- hit:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (e DummySessionExchange) Close() error {
	// NB: exchange doesn't own the blockstore's underlying datastore, so it is
	// not responsible for closing it.
	return nil
}

func edsBlockstore(ctx context.Context, t *testing.T, eds *rsmt2d.ExtendedDataSquare, height uint64) blockstore.Blockstore {
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)

	edsStore, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)

	f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
	require.NoError(t, err)
	f.Close()

	return store.NewBlockstore(edsStore, ds_sync.MutexWrap(datastore.NewMapDatastore()))
}
