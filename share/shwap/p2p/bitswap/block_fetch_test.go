package bitswap

import (
	"context"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/routing/offline"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestFetchDuplicates(t *testing.T) {
	runtime.GOMAXPROCS(3)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	fetcher := fetcher(ctx, t, newTestBlockstore(eds))

	var wg sync.WaitGroup
	for i := range 100 {
		blks := make([]Block, eds.Width())
		for i := range blks {
			blk, err := NewEmptyRowBlock(1, i, root) // create the same Block ID
			require.NoError(t, err)
			blks[i] = blk
		}

		wg.Add(1)
		go func(i int) {
			rint := rand.IntN(10)
			// this sleep ensures fetches aren't started simultaneously allowing to check for edge-cases
			time.Sleep(time.Millisecond * time.Duration(rint))

			err := Fetch(ctx, fetcher, root, blks...)
			assert.NoError(t, err)
			for _, blk := range blks {
				assert.False(t, blk.IsEmpty())
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	var entries int
	populatorFns.Range(func(key, _ any) bool {
		populatorFns.Delete(key)
		entries++
		return true
	})
	require.Zero(t, entries)
}

func fetcher(ctx context.Context, t *testing.T, bstore blockstore.Blockstore) exchange.Fetcher {
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	_ = bitswap.New(
		ctx,
		network.NewFromIpfsHost(net.Hosts()[0], routing),
		bstore,
	)

	dstoreClient := dssync.MutexWrap(ds.NewMapDatastore())
	bstoreClient := blockstore.NewBlockstore(dstoreClient)
	routingClient := offline.NewOfflineRouter(dstoreClient, record.NamespacedValidator{})

	bitswapClient := bitswap.New(
		ctx,
		network.NewFromIpfsHost(net.Hosts()[1], routingClient),
		bstoreClient,
	)

	err = net.ConnectAllButSelf()
	require.NoError(t, err)

	return bitswapClient
}

type testBlockstore struct {
	eds eds.Accessor
}

func newTestBlockstore(rsmt2sEds *rsmt2d.ExtendedDataSquare) *testBlockstore {
	return &testBlockstore{eds: eds.Rsmt2D{ExtendedDataSquare: rsmt2sEds}}
}

func (b *testBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	spec, ok := specRegistry[cid.Prefix().MhType]
	if !ok {
		return nil, fmt.Errorf("unsupported codec")
	}

	bldr, err := spec.builder(cid)
	if err != nil {
		return nil, err
	}

	return bldr.BlockFromEDS(ctx, b.eds)
}

func (b *testBlockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	blk, err := b.Get(ctx, cid)
	if err != nil {
		return 0, err
	}
	return len(blk.RawData()), nil
}

func (b *testBlockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	_, err := b.Get(ctx, cid)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *testBlockstore) Put(context.Context, blocks.Block) error {
	panic("not implemented")
}

func (b *testBlockstore) PutMany(context.Context, []blocks.Block) error {
	panic("not implemented")
}

func (b *testBlockstore) DeleteBlock(context.Context, cid.Cid) error {
	panic("not implemented")
}

func (b *testBlockstore) AllKeysChan(context.Context) (<-chan cid.Cid, error) {
	panic("not implemented")
}

func (b *testBlockstore) HashOnRead(bool) {
	panic("not implemented")
}
