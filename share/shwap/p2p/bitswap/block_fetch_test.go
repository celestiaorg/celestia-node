package bitswap

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/client"
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

	"github.com/celestiaorg/celestia-node/share/eds"
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
			AccessorStreamer: &eds.Rsmt2D{ExtendedDataSquare: rsmt2d},
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
	time.Sleep(time.Millisecond * 10) // give time for connection routines to finish
	return client
}

func newServer(ctx context.Context, host host.Host, store blockstore.Blockstore) {
	net := NewNetwork(host, "test")
	server := NewServer(
		ctx,
		net,
		store,
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
	eds.AccessorStreamer
}

func (t testAccessorGetter) GetByHeight(context.Context, uint64) (eds.AccessorStreamer, error) {
	return t.AccessorStreamer, nil
}

func (t testAccessorGetter) HasByHeight(context.Context, uint64) (bool, error) {
	return true, nil
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

// TestFetch_DuplicateSurvivesOriginalCancel checks that when two Fetches request the same
func TestFetch_DuplicateSurvivesOriginalCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bstore, cids := testBlockstore(ctx, t, 1)
	cid := cids.Keys()[0]

	// servers hold the block back until released, so both requests stay in flight
	gated := &gatedBlockstore{Blockstore: bstore, release: make(chan struct{})}
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	newServer(ctx, net.Hosts()[0], gated)
	exchange := newClient(ctx, net.Hosts()[1], blockstore.NewBlockstore(ds.NewMapDatastore()))
	require.NoError(t, net.ConnectAllButSelf())
	time.Sleep(time.Millisecond * 10)

	blkA, err := newEmptyTestBlock(cid)
	require.NoError(t, err)
	ctxA, cancelA := context.WithCancel(ctx)
	doneA := make(chan error, 1)
	go func() { doneA <- Fetch(ctxA, exchange, nil, []Block{blkA}) }()
	require.Eventually(t, func() bool {
		_, ok := unmarshalFns.Load(cid)
		return ok
	}, time.Second, time.Millisecond)

	blkB, err := newEmptyTestBlock(cid)
	require.NoError(t, err)
	ctxB, cancelB := context.WithTimeout(ctx, time.Second*3)
	defer cancelB()
	requestedB := make(chan struct{})
	fetcherB := &signalFetcher{Embedded: exchange, requested: requestedB}
	doneB := make(chan error, 1)
	go func() { doneB <- Fetch(ctxB, exchange, nil, []Block{blkB}, WithFetcher(fetcherB)) }()

	select {
	case <-requestedB:
	case <-ctx.Done():
		t.Fatal("timeout waiting for duplicate fetch to request block")
	}

	// the original requester gives up before the block arrives
	cancelA()
	select {
	case err := <-doneA:
		require.ErrorIs(t, err, context.Canceled)
	case <-ctx.Done():
		t.Fatal("timeout waiting for original fetch to return")
	}

	close(gated.release)
	select {
	case err := <-doneB:
		require.NoError(t, err, "duplicate Fetch must receive the block after the original requester canceled")
	case <-ctx.Done():
		t.Fatal("timeout waiting for duplicate fetch to return")
	}
	require.NotEmpty(t, blkB.data)
	_, ok := unmarshalFns.Load(cid)
	require.False(t, ok, "unmarshalFns should be cleaned up after all fetches complete")
}

type gatedBlockstore struct {
	blockstore.Blockstore
	release chan struct{}
}

func (g *gatedBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	select {
	case <-g.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return g.Blockstore.Get(ctx, c)
}

type signalFetcher struct {
	Embedded  exchange.Fetcher
	requested chan struct{}
}

func (s *signalFetcher) GetBlock(context.Context, cid.Cid) (blocks.Block, error) {
	panic("not implemented")
}

func (s *signalFetcher) GetBlocks(ctx context.Context, cids []cid.Cid) (<-chan blocks.Block, error) {
	ch, err := s.Embedded.GetBlocks(ctx, cids)
	close(s.requested)
	return ch, err
}
