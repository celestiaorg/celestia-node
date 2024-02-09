package getters

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/rand"

	"github.com/celestiaorg/celestia-app/pkg/da"
	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/store"
)

func TestShrexGetter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	clHost, srvHost := net.Hosts()[0], net.Hosts()[1]

	// launch eds store and put test data into it
	edsStore, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	height := atomic.Uint64{}

	ndClient, _ := newNDClientServer(ctx, t, edsStore, srvHost, clHost)
	edsClient, _ := newEDSClientServer(ctx, t, edsStore, srvHost, clHost)

	// create shrex Getter
	sub := new(headertest.Subscriber)
	peerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)
	getter := NewShrexGetter(edsClient, ndClient, peerManager)
	require.NoError(t, getter.Start(ctx))

	t.Run("ND_Available, total data size > 1mb", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		// generate test data
		size := 128
		namespace := sharetest.RandV0Namespace()
		sqSise := size * size
		eds, dah := edstest.RandEDSWithNamespace(t, namespace, sqSise/2+rand.Intn(sqSise/2), size)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		defer f.Close()
		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		got, err := getter.GetSharesByNamespace(ctx, eh, namespace)
		require.NoError(t, err)
		require.NoError(t, got.Verify(dah, namespace))
	})

	t.Run("ND_err_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		_, dah, namespace := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)

		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		_, err := getter.GetSharesByNamespace(ctx, eh, namespace)
		require.ErrorIs(t, err, share.ErrNotFound)
	})

	t.Run("ND_namespace_not_included", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*444)
		t.Cleanup(cancel)

		// generate test data
		eds, dah, maxNamespace := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		nID, err := maxNamespace.AddInt(-1)
		require.NoError(t, err)
		// check for namespace to be between max and min namespace in root
		require.Len(t, ipld.FilterRootByNamespace(dah, nID), 1)

		sgetter := NewStoreGetter(edsStore)
		emptyShares, err := sgetter.GetSharesByNamespace(ctx, eh, nID)
		require.NoError(t, err)
		// no shares should be returned
		require.Empty(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(dah, nID))

		emptyShares1, err := getter.GetSharesByNamespace(ctx, eh, nID)
		require.Equal(t, emptyShares.Flatten(), emptyShares1.Flatten())
		require.NoError(t, err)
		// no shares should be returned
		require.Empty(t, emptyShares1.Flatten())
		require.Nil(t, emptyShares1.Verify(dah, nID))
	})

	t.Run("ND_namespace_not_in_dah", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		eds, dah, maxNamesapce := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		namespace, err := maxNamesapce.AddInt(1)
		require.NoError(t, err)
		// check for namespace to be not in root
		require.Len(t, ipld.FilterRootByNamespace(dah, namespace), 0)

		emptyShares, err := getter.GetSharesByNamespace(ctx, eh, namespace)
		require.NoError(t, err)
		// no shares should be returned
		require.Empty(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(dah, namespace))
	})

	t.Run("EDS_Available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		eds, dah, _ := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		got, err := getter.GetEDS(ctx, eh)
		require.NoError(t, err)
		require.True(t, got.Equals(eds))
	})

	t.Run("EDS get empty", func(t *testing.T) {
		// empty root
		emptyRoot := da.MinDataAvailabilityHeader()
		eh := headertest.RandExtendedHeaderWithRoot(t, &emptyRoot)

		eds, err := getter.GetEDS(ctx, eh)
		require.NoError(t, err)
		dah, err := share.NewRoot(eds)
		require.NoError(t, err)
		require.True(t, share.DataHash(dah.Hash()).IsEmptyRoot())
	})

	t.Run("EDS_ctx_deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)

		// generate test data
		_, dah, _ := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		cancel()
		_, err := getter.GetEDS(ctx, eh)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("EDS_err_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		_, dah, _ := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, dah)
		height := height.Add(1)
		eh.RawHeader.Height = int64(height)

		peerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: dah.Hash(),
			Height:   height,
		})

		_, err := getter.GetEDS(ctx, eh)
		require.ErrorIs(t, err, share.ErrNotFound)
	})
}

func generateTestEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.Root, share.Namespace) {
	eds := edstest.RandEDS(t, 4)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	max := nmt.MaxNamespace(dah.RowRoots[(len(dah.RowRoots))/2-1], share.NamespaceSize)
	return eds, dah, max
}

func testManager(
	ctx context.Context, host host.Host, headerSub libhead.Subscriber[*header.ExtendedHeader],
) (*peers.Manager, error) {
	shrexSub, err := shrexsub.NewPubSub(ctx, host, "test")
	if err != nil {
		return nil, err
	}

	connGater, err := conngater.NewBasicConnectionGater(ds_sync.MutexWrap(datastore.NewMapDatastore()))
	if err != nil {
		return nil, err
	}
	manager, err := peers.NewManager(
		peers.DefaultParameters(),
		host,
		connGater,
		peers.WithShrexSubPools(shrexSub, headerSub),
	)
	return manager, err
}

func newNDClientServer(
	ctx context.Context, t *testing.T, edsStore *store.Store, srvHost, clHost host.Host,
) (*shrexnd.Client, *shrexnd.Server) {
	params := shrexnd.DefaultParameters()

	// create server and register handler
	server, err := shrexnd.NewServer(params, srvHost, edsStore)
	require.NoError(t, err)
	require.NoError(t, server.Start(ctx))

	t.Cleanup(func() {
		_ = server.Stop(ctx)
	})

	// create client and connect it to server
	client, err := shrexnd.NewClient(params, clHost)
	require.NoError(t, err)
	return client, server
}

func newEDSClientServer(
	ctx context.Context, t *testing.T, edsStore *store.Store, srvHost, clHost host.Host,
) (*shrexeds.Client, *shrexeds.Server) {
	params := shrexeds.DefaultParameters()

	// create server and register handler
	server, err := shrexeds.NewServer(params, srvHost, edsStore)
	require.NoError(t, err)
	require.NoError(t, server.Start(ctx))

	t.Cleanup(func() {
		_ = server.Stop(ctx)
	})

	// create client and connect it to server
	client, err := shrexeds.NewClient(params, clHost)
	require.NoError(t, err)
	return client, server
}
