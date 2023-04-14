package getters

import (
	"context"
	mrand "math/rand"
	"testing"
	"time"

	bsrv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/discovery"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

func TestGetSharesWithProofByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net := availability_test.NewTestDAGNet(ctx, t)

	// launch eds store and put test data into it
	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	// create server and register handler
	bServ := mdutils.Bserv()
	srvHost := net.NewTestNode().Host
	params := shrexnd.DefaultParameters()
	srv, err := shrexnd.NewServer(params, srvHost, edsStore, NewIPLDGetter(bServ))
	require.NoError(t, err)
	require.NoError(t, srv.Start(ctx))

	t.Cleanup(func() {
		_ = srv.Stop(ctx)
	})

	// create client and connect it to server
	clHost := net.NewTestNode().Host
	client, err := shrexnd.NewClient(params, clHost)
	require.NoError(t, err)
	net.ConnectAll()

	// create shrex Getter
	sub := new(headertest.DummySubscriber)
	peermanager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)
	getter := NewShrexGetter(nil, client, peermanager)
	require.NoError(t, getter.Start(ctx))

	t.Run("ND_Available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		// generate test data
		randomEDS, nID := generateTestEDS(t, bServ)
		dah := da.NewDataAvailabilityHeader(randomEDS)
		require.NoError(t, edsStore.Put(ctx, dah.Hash(), randomEDS))
		peermanager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   1,
		})

		got, err := getter.GetSharesByNamespace(ctx, &dah, nID)
		require.NoError(t, err)
		require.NoError(t, got.Verify(&dah, nID))
	})

	t.Run("ND_err_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		// generate test data
		randomEDS, nID := generateTestEDS(t, bServ)
		dah := da.NewDataAvailabilityHeader(randomEDS)
		peermanager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   1,
		})

		_, err := getter.GetSharesByNamespace(ctx, &dah, nID)
		require.ErrorIs(t, err, share.ErrNotFound)
	})
}

func newStore(t *testing.T) (*eds.Store, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return eds.NewStore(tmpDir, ds)
}

func generateTestEDS(t *testing.T, bServ bsrv.BlockService) (*rsmt2d.ExtendedDataSquare, namespace.ID) {
	shares := share.RandShares(t, 16)

	from := mrand.Intn(len(shares))
	to := mrand.Intn(len(shares))

	if to < from {
		from, to = to, from
	}

	nID := shares[from][:share.NamespaceSize]
	// change some shares to have same nID
	for i := from; i <= to; i++ {
		copy(shares[i][:share.NamespaceSize], nID)
	}

	eds, err := share.AddShares(context.Background(), shares, bServ)
	require.NoError(t, err)

	return eds, nID
}

func testManager(ctx context.Context, host host.Host, headerSub libhead.Subscriber[*header.ExtendedHeader]) (*peers.Manager, error) {
	shrexSub, err := shrexsub.NewPubSub(ctx, host, "test")
	if err != nil {
		return nil, err
	}

	disc := discovery.NewDiscovery(nil,
		routingdisc.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	connGater, err := conngater.NewBasicConnectionGater(ds_sync.MutexWrap(datastore.NewMapDatastore()))
	if err != nil {
		return nil, err
	}
	manager, err := peers.NewManager(
		peers.DefaultParameters(),
		headerSub,
		shrexSub,
		disc,
		host,
		connGater,
	)
	return manager, err
}
