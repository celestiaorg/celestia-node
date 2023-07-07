package shrexnd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/p2p"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestExchange_RequestND_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	edsStore, client, server := makeExchange(t, notFoundGetter{})
	require.NoError(t, edsStore.Start(ctx))
	require.NoError(t, server.Start(ctx))

	t.Run("CAR_not_exist", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		root := share.Root{}
		namespace := sharetest.RandV0Namespace()
		_, err := client.RequestND(ctx, &root, namespace, server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
	})

	t.Run("ErrNamespaceNotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds := edstest.RandEDS(t, 4)
		dah := da.NewDataAvailabilityHeader(eds)
		require.NoError(t, edsStore.Put(ctx, dah.Hash(), eds))

		randNamespace := dah.RowRoots[(len(dah.RowRoots)-1)/2][:share.NamespaceSize]
		emptyShares, err := client.RequestND(ctx, &dah, randNamespace, server.host.ID())
		require.NoError(t, err)
		require.Empty(t, emptyShares.Flatten())
	})
}

func TestExchange_RequestND(t *testing.T) {
	t.Run("ND_concurrency_limit", func(t *testing.T) {
		net, err := mocknet.FullMeshConnected(2)
		require.NoError(t, err)

		client, err := NewClient(DefaultParameters(), net.Hosts()[0])
		require.NoError(t, err)
		server, err := NewServer(DefaultParameters(), net.Hosts()[1], nil, nil)
		require.NoError(t, err)

		require.NoError(t, server.Start(context.Background()))

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		t.Cleanup(cancel)

		rateLimit := 2
		wg := sync.WaitGroup{}
		wg.Add(rateLimit)

		// mockHandler will block requests on server side until test is over
		lock := make(chan struct{})
		defer close(lock)
		mockHandler := func(network.Stream) {
			wg.Done()
			select {
			case <-lock:
			case <-ctx.Done():
				t.Fatal("timeout")
			}
		}
		middleware := p2p.NewMiddleware(rateLimit)
		server.host.SetStreamHandler(server.protocolID,
			middleware.RateLimitHandler(mockHandler))

		// take server concurrency slots with blocked requests
		for i := 0; i < rateLimit; i++ {
			go func(i int) {
				client.RequestND(ctx, nil, sharetest.RandV0Namespace(), server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestND(ctx, nil, sharetest.RandV0Namespace(), server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
	})
}

type notFoundGetter struct{}

func (m notFoundGetter) GetShare(
	_ context.Context, _ *share.Root, _, _ int,
) (share.Share, error) {
	return nil, share.ErrNotFound
}

func (m notFoundGetter) GetEDS(
	_ context.Context, _ *share.Root,
) (*rsmt2d.ExtendedDataSquare, error) {
	return nil, share.ErrNotFound
}

func (m notFoundGetter) GetSharesByNamespace(
	_ context.Context, _ *share.Root, _ share.Namespace,
) (share.NamespacedShares, error) {
	return nil, nil
}

func newStore(t *testing.T) *eds.Store {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	store, err := eds.NewStore(tmpDir, ds)
	require.NoError(t, err)
	return store
}

func createMocknet(t *testing.T, amount int) []libhost.Host {
	t.Helper()

	net, err := mocknet.FullMeshConnected(amount)
	require.NoError(t, err)
	// get host and peer
	return net.Hosts()
}

func makeExchange(t *testing.T, getter share.Getter) (*eds.Store, *Client, *Server) {
	t.Helper()
	store := newStore(t)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultParameters(), hosts[1], store, getter)
	require.NoError(t, err)

	return store, client, server
}
