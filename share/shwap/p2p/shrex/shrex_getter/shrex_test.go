package shrex_getter //nolint:stylecheck // underscore in pkg name will be fixed with shrex refactoring

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

func TestShrexGetter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)
	clHost, srvHost := net.Hosts()[0], net.Hosts()[1]

	// launch eds store and put test data into it
	edsStore, err := newStore(t)
	require.NoError(t, err)

	ndClient, _ := newNDClientServer(ctx, t, edsStore, srvHost, clHost)

	// create shrex Getter
	sub := new(headertest.Subscriber)

	fullPeerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)
	archivalPeerManager, err := testManager(ctx, clHost, sub)
	require.NoError(t, err)

	getter := NewGetter(ndClient, fullPeerManager, archivalPeerManager, availability.RequestWindow)
	require.NoError(t, getter.Start(ctx))

	height := atomic.Uint64{}
	height.Add(1)

	t.Run("ND_Available, total data size > 1mb", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		t.Cleanup(cancel)

		// generate test data
		size := 64
		namespace := libshare.RandomNamespace()
		height := height.Add(1)
		randEDS, roots := edstest.RandEDSWithNamespace(t, namespace, size*size, size)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		got, err := getter.GetNamespaceData(ctx, eh, namespace)
		require.NoError(t, err)
		require.NoError(t, got.Verify(roots, namespace))
	})

	t.Run("ND_err_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		height := height.Add(1)
		_, roots, namespace := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		_, err := getter.GetNamespaceData(ctx, eh, namespace)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})

	t.Run("ND_namespace_not_included", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		height := height.Add(1)
		eds, roots, maxNamespace := generateTestEDS(t)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		// namespace inside root range
		nID, err := maxNamespace.AddInt(-1)
		require.NoError(t, err)
		// check for namespace to be between max and min namespace in root
		rows, err := share.RowsWithNamespace(roots, nID)
		require.NoError(t, err)
		require.Len(t, rows, 1)

		emptyShares, err := getter.GetNamespaceData(ctx, eh, nID)
		require.NoError(t, err)
		// no shares should be returned
		require.Nil(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(roots, nID))

		// namespace outside root range
		nID, err = maxNamespace.AddInt(1)
		require.NoError(t, err)
		// check for namespace to be not in root
		rows, err = share.RowsWithNamespace(roots, nID)
		require.NoError(t, err)
		require.Len(t, rows, 0)

		emptyShares, err = getter.GetNamespaceData(ctx, eh, nID)
		require.NoError(t, err)
		// no shares should be returned
		require.Nil(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(roots, nID))
	})

	t.Run("ND_namespace_not_in_dah", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		eds, roots, maxNamespace := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		namespace, err := maxNamespace.AddInt(1)
		require.NoError(t, err)
		// check for namespace to be not in root
		rows, err := share.RowsWithNamespace(roots, namespace)
		require.NoError(t, err)
		require.Len(t, rows, 0)

		emptyShares, err := getter.GetNamespaceData(ctx, eh, namespace)
		require.NoError(t, err)
		// no shares should be returned
		require.Empty(t, emptyShares.Flatten())
		require.Nil(t, emptyShares.Verify(roots, namespace))
	})

	t.Run("EDS_Available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		// generate test data
		randEDS, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		err = edsStore.PutODSQ4(ctx, roots, height, randEDS)
		require.NoError(t, err)
		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		got, err := getter.GetEDS(ctx, eh)
		require.NoError(t, err)
		require.Equal(t, randEDS.Flattened(), got.Flattened())
	})

	t.Run("EDS_ctx_deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)

		// generate test data
		_, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
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
		_, roots, _ := generateTestEDS(t)
		height := height.Add(1)
		eh := headertest.RandExtendedHeaderWithRoot(t, roots)
		eh.RawHeader.Height = int64(height)

		fullPeerManager.Validate(ctx, srvHost.ID(), shrexsub.Notification{
			DataHash: roots.Hash(),
			Height:   height,
		})

		_, err := getter.GetEDS(ctx, eh)
		require.ErrorIs(t, err, shwap.ErrNotFound)
	})

	// tests getPeer's ability to route requests based on whether
	// they are historical or not
	t.Run("routing_historical_requests", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		archivalPeer, err := net.GenPeer()
		require.NoError(t, err)
		fullPeer, err := net.GenPeer()
		require.NoError(t, err)

		getter.archivalPeerManager.UpdateNodePool(archivalPeer.ID(), true)
		getter.fullPeerManager.UpdateNodePool(fullPeer.ID(), true)

		height := height.Add(1)
		eh := headertest.RandExtendedHeader(t)
		eh.RawHeader.Height = int64(height)

		// historical data expects an archival peer
		eh.RawHeader.Time = time.Now().Add(-(availability.StorageWindow + time.Second))
		id, _, err := getter.getPeer(ctx, eh)
		require.NoError(t, err)
		assert.Equal(t, archivalPeer.ID(), id)

		// recent (within sampling window) data expects a full peer
		eh.RawHeader.Time = time.Now()
		id, _, err = getter.getPeer(ctx, eh)
		require.NoError(t, err)
		assert.Equal(t, fullPeer.ID(), id)
	})
}

func newStore(t *testing.T) (*store.Store, error) {
	t.Helper()

	return store.NewStore(store.DefaultParameters(), t.TempDir())
}

func generateTestEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots, libshare.Namespace) {
	eds := edstest.RandEDS(t, 4)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	max := nmt.MaxNamespace(roots.RowRoots[(len(roots.RowRoots))/2-1], libshare.NamespaceSize)
	ns, err := libshare.NewNamespaceFromBytes(max)
	require.NoError(t, err)
	return eds, roots, ns
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
		*peers.DefaultParameters(),
		host,
		connGater,
		"test",
		peers.WithShrexSubPools(shrexSub, headerSub),
	)
	return manager, err
}

func newNDClientServer(
	ctx context.Context, t *testing.T, edsStore *store.Store, srvHost, clHost host.Host,
) (*shrexnd.Client, *shrexnd.Server) {
	params := shrexnd.DefaultParameters()

	// create server and register handler
	server, err := shrexnd.NewServer(params, srvHost, edsStore, shrexnd.SupportedProtocols())
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
