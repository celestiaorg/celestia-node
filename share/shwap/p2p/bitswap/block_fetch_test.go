package bitswap

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/bitswap/client"
	"github.com/ipfs/boxo/bitswap/network"
	"github.com/ipfs/boxo/bitswap/server"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/boxo/routing/offline"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestFetchDuplicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	eds := edstest.RandEDS(t, 4)
	root, err := share.NewRoot(eds)
	require.NoError(t, err)
	fetcher := fetcher(ctx, t, eds)

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
			// this sleep ensures fetches aren't started simultaneously, allowing to check for edge-cases
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
	unmarshalFns.Range(func(key, _ any) bool {
		unmarshalFns.Delete(key)
		entries++
		return true
	})
	require.Zero(t, entries)
}

func fetcher(ctx context.Context, t *testing.T, rsmt2dEds *rsmt2d.ExtendedDataSquare) exchange.SessionExchange {
	bstore := &Blockstore{
		Accessors: testAccessors{
			Accessor: eds.Rsmt2D{ExtendedDataSquare: rsmt2dEds},
		},
	}

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
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	net := network.NewFromIpfsHost(host, routing)
	server := server.New(
		ctx,
		net,
		store,
		server.TaskWorkerCount(2),
		server.EngineTaskWorkerCount(2),
		server.ProvideEnabled(false),
		server.SetSendDontHaves(false),
	)
	net.Start(server)
}

func newClient(ctx context.Context, host host.Host, store blockstore.Blockstore) *client.Client {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	net := network.NewFromIpfsHost(host, routing)
	client := client.New(
		ctx,
		net,
		store,
	)
	net.Start(client)
	return client
}

type testAccessors struct {
	eds.Accessor
}

func (t testAccessors) Get(context.Context, uint64) (eds.Accessor, error) {
	return t.Accessor, nil
}
