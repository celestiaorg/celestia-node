package bitswap

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/server"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/rsmt2d"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

func TestFetch_Options(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	const items = 128
	bstore, cids := testBlockstore(ctx, t, items)

	t.Run("WithBlockstore", func(t *testing.T) {
		exchange := newExchange(ctx, t, bstore)

		blks := make([]Block, 0, cids.Len())
		_ = cids.ForEach(func(c cid.Cid) error {
			blk, err := newEmptyTestBlock(c)
			require.NoError(t, err)
			blks = append(blks, blk)
			return nil
		})

		bstore := blockstore.NewBlockstore(ds.NewMapDatastore())
		err := Fetch(ctx, exchange, nil, blks, WithStore(bstore))
		require.NoError(t, err)

		for _, blk := range blks {
			ok, err := bstore.Has(ctx, blk.CID())
			require.NoError(t, err)
			require.True(t, ok)
		}
	})

	t.Run("WithFetcher", func(t *testing.T) {
		exchange := newExchange(ctx, t, bstore)

		blks := make([]Block, 0, cids.Len())
		_ = cids.ForEach(func(c cid.Cid) error {
			blk, err := newEmptyTestBlock(c)
			require.NoError(t, err)
			blks = append(blks, blk)
			return nil
		})

		session := exchange.NewSession(ctx)
		fetcher := &testFetcher{Embedded: session}
		err := Fetch(ctx, exchange, nil, blks, WithFetcher(fetcher))
		require.NoError(t, err)
		require.Equal(t, len(blks), fetcher.Fetched)
	})
}

func TestFetch_Duplicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	const items = 128
	bstore, cids := testBlockstore(ctx, t, items)
	exchange := newExchange(ctx, t, bstore)

	var wg sync.WaitGroup
	for i := range items {
		blks := make([]Block, 0, cids.Len())
		_ = cids.ForEach(func(c cid.Cid) error {
			blk, err := newEmptyTestBlock(c)
			require.NoError(t, err)
			blks = append(blks, blk)
			return nil
		})

		wg.Add(1)
		go func(i int) {
			rint := rand.IntN(10)
			// this sleep ensures fetches aren't started simultaneously, allowing to check for edge-cases
			time.Sleep(time.Millisecond * time.Duration(rint))

			err := Fetch(ctx, exchange, nil, blks)
			assert.NoError(t, err)
			wg.Done()
		}(i)
	}
	wg.Wait()

	var entries int
	unmarshalFns.Range(func(key, _ any) bool {
		unmarshalFns.Delete(key)
		entries++
		return true
	})
	require.Zero(t, entries)
}

func newExchangeOverEDS(ctx context.Context, t *testing.T, rsmt2d *rsmt2d.ExtendedDataSquare) exchange.SessionExchange {
	bstore := &Blockstore{
		Getter: testAccessorGetter{
			Accessor: eds.Rsmt2D{ExtendedDataSquare: rsmt2d},
		},
	}
	return newExchange(ctx, t, bstore)
}

func newExchange(ctx context.Context, t *testing.T, bstore blockstore.Blockstore) exchange.SessionExchange {
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	newServer(ctx, net.Hosts()[0], bstore)
	newServer(ctx, net.Hosts()[1], bstore)

	client := newClient(ctx, net.Hosts()[2], bstore)

	err = net.ConnectAllButSelf()
	require.NoError(t, err)
	return client
}

func newServer(ctx context.Context, host host.Host, store blockstore.Blockstore) {
	net := NewNetwork(host, "test")
	server := NewServer(
		ctx,
		net,
		store,
		server.TaskWorkerCount(2),
		server.EngineTaskWorkerCount(2),
	)
	net.Start(server)
}

func newClient(ctx context.Context, host host.Host, store blockstore.Blockstore) *client.Client {
	net := NewNetwork(host, "test")
	client := NewClient(ctx, net, store)
	net.Start(client)
	return client
}

type testAccessorGetter struct {
	eds.Accessor
}

func (t testAccessorGetter) GetByHeight(context.Context, uint64) (eds.Accessor, error) {
	return t.Accessor, nil
}

type testFetcher struct {
	Fetched int

	Embedded exchange.Fetcher
}

func (t *testFetcher) GetBlock(context.Context, cid.Cid) (blocks.Block, error) {
	panic("not implemented")
}

func (t *testFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (<-chan blocks.Block, error) {
	t.Fetched += len(cids)
	return t.Embedded.GetBlocks(ctx, cids)
}
