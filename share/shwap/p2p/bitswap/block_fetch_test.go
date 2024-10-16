package bitswap

import (
	"context"
	"encoding/json"
	"github.com/ipfs/boxo/bitswap/message"
	pb "github.com/ipfs/boxo/bitswap/message/pb"
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

func TestFetch_List(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	const items = 128
	bstore, cids := testBlockstore(ctx, t, items)
	exchange := newExchange(ctx, t, bstore, true)

	blks := make([]Block, 0, cids.Len())
	_ = cids.ForEach(func(c cid.Cid) error {
		blk, err := newEmptyTestBlock(c)
		require.NoError(t, err)
		blks = append(blks, blk)
		return nil
	})

	go func() {
		_ = Fetch(ctx, exchange, nil, blks)
	}()

	time.Sleep(time.Millisecond * 100)

	awaiting := ListActiveFetches()
	for _, cid := range awaiting {
		assert.True(t, cids.Has(cid))
	}
	assert.Equal(t, cids.Len(), len(awaiting))
}

func TestFetch_Rerequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*500)
	defer cancel()

	const items = 128
	bstore, cids := testBlockstore(ctx, t, items)
	exchange := newExchange(ctx, t, bstore, true)

	blks := make([]Block, 0, cids.Len())
	_ = cids.ForEach(func(c cid.Cid) error {
		blk, err := newEmptyTestBlock(c)
		require.NoError(t, err)
		blks = append(blks, blk)
		return nil
	})

	ses := exchange.NewSession(ctx)
	fetchCtx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()
	_ = Fetch(fetchCtx, exchange, nil, blks, WithFetcher(ses))

	time.Sleep(time.Millisecond * 100)

	_ = Fetch(ctx, exchange, nil, blks, WithFetcher(ses))

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

func newExchange(
	ctx context.Context,
	t *testing.T,
	bstore blockstore.Blockstore,
	stuck ...bool,
) exchange.SessionExchange {
	net, err := mocknet.FullMeshLinked(3)
	require.NoError(t, err)

	newServer(ctx, net.Hosts()[0], bstore)
	newServer(ctx, net.Hosts()[1], bstore)

	client := newClient(ctx, net.Hosts()[2], bstore)

	if len(stuck) > 0 && stuck[0] {
		return client
	}

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

func Test(t *testing.T) {
	// timeout
	symbolsA := `[
    "bagipaamr6aaqyaaaaaaaaaaffyaesaa7",
    "bagipaamr6aaqyaaaaaaaaaafhyaekadt",
    "bagipaamr6aaqyaaaaaaaaaafimagoad7",
    "bagipaamr6aaqyaaaaaaaaaafiaagkaad",
    "bagipaamr6aaqyaaaaaaaaaafimaboadm",
    "bagipaamr6aaqyaaaaaaaaaafhyabmadz",
    "bagipaamr6aaqyaaaaaaaaaafhyagiact",
    "bagipaamr6aaqyaaaaaaaaaafhyae4acc",
    "bagipaamr6aaqyaaaaaaaaaaffyaasaar",
    "bagipaamr6aaqyaaaaaaaaaafiaacqabr",
    "bagipaamr6aaqyaaaaaaaaaafimad4abc",
    "bagipaamr6aaqyaaaaaaaaaaffyad6aag",
    "bagipaamr6aaqyaaaaaaaaaafimaeaacd",
    "bagipaamr6aaqyaaaaaaaaaafiaac4add",
    "bagipaamr6aaqyaaaaaaaaaafiaafuadz",
    "bagipaamr6aaqyaaaaaaaaaafiaah2adv",
    "bagipaamr6aaqyaaaaaaaaaafiaacuaac",
    "bagipaamr6aaqyaaaaaaaaaafhyaawad4",
    "bagipaamr6aaqyaaaaaaaaaafiaadwady",
    "bagipaamr6aaqyaaaaaaaaaafimadaaa2",
    "bagipaamr6aaqyaaaaaaaaaafhyadwabi",
    "bagipaamr6aaqyaaaaaaaaaafiaaaiaai",
    "bagipaamr6aaqyaaaaaaaaaaffyah4act",
    "bagipaamr6aaqyaaaaaaaaaafiaadyadi",
    "bagipaamr6aaqyaaaaaaaaaaffyagsaax",
    "bagipaamr6aaqyaaaaaaaaaafimaciaac",
    "bagipaamr6aaqyaaaaaaaaaaffyab4abn",
    "bagipaamr6aaqyaaaaaaaaaafiaaa4abr",
    "bagipaamr6aaqyaaaaaaaaaafiaaegabj",
    "bagipaamr6aaqyaaaaaaaaaafiaag6aay",
    "bagipaamr6aaqyaaaaaaaaaafiaaciaab",
    "bagipaamr6aaqyaaaaaaaaaaffyaekacc",
    "bagipaamr6aaqyaaaaaaaaaaffyacuaay",
    "bagipaamr6aaqyaaaaaaaaaafimaeaaaf",
    "bagipaamr6aaqyaaaaaaaaaafhyadkabq",
    "bagipaamr6aaqyaaaaaaaaaafhyaf4aav",
    "bagipaamr6aaqyaaaaaaaaaaffyadyaaf",
    "bagipaamr6aaqyaaaaaaaaaaffyabsaa7",
    "bagipaamr6aaqyaaaaaaaaaaffyafuab7",
    "bagipaamr6aaqyaaaaaaaaaafimaagaaq",
    "bagipaamr6aaqyaaaaaaaaaafiaabuab3",
    "bagipaamr6aaqyaaaaaaaaaafhyag2abp",
    "bagipaamr6aaqyaaaaaaaaaafimaauabo",
    "bagipaamr6aaqyaaaaaaaaaaffyacuabb",
    "bagipaamr6aaqyaaaaaaaaaafhyaheacf",
    "bagipaamr6aaqyaaaaaaaaaafimagyad6",
    "bagipaamr6aaqyaaaaaaaaaafhyagsac3",
    "bagipaamr6aaqyaaaaaaaaaafimaeeab7",
    "bagipaamr6aaqyaaaaaaaaaaffyadeacf",
    "bagipaamr6aaqyaaaaaaaaaafhyaeeaau",
    "bagipaamr6aaqyaaaaaaaaaaffyabgadv",
    "bagipaamr6aaqyaaaaaaaaaafhyad2adf",
    "bagipaamr6aaqyaaaaaaaaaafimag4aaf",
    "bagipaamr6aaqyaaaaaaaaaafiaadiabi",
    "bagipaamr6aaqyaaaaaaaaaafimaewaai",
    "bagipaamr6aaqyaaaaaaaaaafhyaaaad2",
    "bagipaamr6aaqyaaaaaaaaaafimagqaah",
    "bagipaamr6aaqyaaaaaaaaaafhyaawadb",
    "bagipaamr6aaqyaaaaaaaaaafiaabuac5",
    "bagipaamr6aaqyaaaaaaaaaaffyadsaaw",
    "bagipaamr6aaqyaaaaaaaaaaffyaasadu",
    "bagipaamr6aaqyaaaaaaaaaafhyab6aai",
    "bagipaamr6aaqyaaaaaaaaaafimafqaby",
    "bagipaamr6aaqyaaaaaaaaaafimahmaam"
  ]`

	// unmarshallers
	symbolsB := `[
        "bagipaamr6aaqyaaaaaaaaa5hkeaaoab7",
        "bagipaamr6aaqyaaaaaaaaa7yaeae2aay",
        "bagipaamr6aaqyaaaaaaaaa73rmafcabv",
        "bagipaamr6aaqyaaaaaaaaa5d24afcadh",
        "bagipaamr6aaqyaaaaaaaaa5etuabgadq",
        "bagipaamr6aaqyaaaaaaaaa7zruagoaa4",
        "bagipaamr6aaqyaaaaaaaaa73rmaeqabo",
        "bagipaamr6aaqyaaaaaaaaa5ehiaegaar",
        "bagipaamr6aaqyaaaaaaaaa5etuabaadp"
    ]`

	var idsA, idsB []string
	json.Unmarshal([]byte(symbolsA), &idsA)
	json.Unmarshal([]byte(symbolsB), &idsB)

	t.Log("A: ", len(idsA))
	var cidsA []cid.Cid
	for _, id := range idsA {
		cid, _ := cid.Decode(id)
		cidsA = append(cidsA, cid)
		blk, _ := EmptyBlock(cid)
		t.Log(blk.Height(), blk.(*SampleBlock).ID.RowIndex, blk.(*SampleBlock).ID.ShareIndex)
	}

	msg := message.New(false)

	var size int
	for _, id := range cidsA {
		size += msg.AddEntry(id, 1, pb.Message_Wantlist_Have, true)
	}

	t.Log(size)

	//t.Log("PAUSE")
	//for _, id := range idsB {
	//	cid, _ := cid.Decode(id)
	//	blk, _ := EmptyBlock(cid)
	//	t.Log(blk.Height(), blk.(*SampleBlock).ID.RowIndex, blk.(*SampleBlock).ID.ShareIndex)
	//}
	//

}
