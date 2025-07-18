package shrex

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store"
)

func TestExchange_RequestND_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	edsStore, client, server := makeExchange(t)

	height := atomic.Uint64{}
	height.Add(1)

	t.Run("CAR_not_exist", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		namespace := libshare.RandomNamespace()
		height := height.Add(1)

		id, err := shwap.NewNamespaceDataID(height, namespace)
		data := shwap.NamespaceData{}
		require.NoError(t, err)
		err = client.Get(ctx, &id, &data, server.host.ID())
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("ErrNamespaceNotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)

		height := height.Add(1)
		err = edsStore.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)

		namespace := libshare.RandomNamespace()

		id, err := shwap.NewNamespaceDataID(height, namespace)
		data := shwap.NamespaceData{}
		require.NoError(t, err)

		err = client.Get(ctx, &id, &data, server.host.ID())
		require.NoError(t, err)
		require.Empty(t, data.Flatten())
	})
}

func TestExchange_RequestND(t *testing.T) {
	t.Run("ND_concurrency_limit", func(t *testing.T) {
		net, err := mocknet.FullMeshConnected(2)
		require.NoError(t, err)

		client, err := NewClient(DefaultClientParameters(), net.Hosts()[0])
		require.NoError(t, err)
		server, err := NewServer(DefaultServerParameters(), net.Hosts()[1], nil)
		require.NoError(t, err)

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
		middleware, err := newMiddleware(rateLimit)
		require.NoError(t, err)
		for _, reqID := range registry {
			server.host.SetStreamHandler(
				ProtocolID(server.params.NetworkID(), reqID().Name()),
				middleware.rateLimitHandler(
					ctx,
					mockHandler,
					reqID().Name(),
				),
			)
		}

		// take server concurrency slots with blocked requests
		for i := range rateLimit {
			go func(i int) {
				namespace := libshare.RandomNamespace()
				id, err := shwap.NewNamespaceDataID(1, namespace)
				data := shwap.NamespaceData{}
				require.NoError(t, err)

				client.Get(ctx, &id, &data, server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		namespace := libshare.RandomNamespace()
		id, err := shwap.NewNamespaceDataID(1, namespace)
		data := shwap.NamespaceData{}
		require.NoError(t, err)

		err = client.Get(ctx, &id, &data, server.host.ID())
		require.ErrorIs(t, err, ErrRateLimited)
	})
}

func createMocknet(t *testing.T, amount int) []libhost.Host {
	t.Helper()

	net, err := mocknet.FullMeshConnected(amount)
	require.NoError(t, err)
	// get host and peer
	return net.Hosts()
}

func makeExchange(t *testing.T) (*store.Store, *Client, *Server) {
	t.Helper()
	s, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultClientParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultServerParameters(), hosts[1], s)
	require.NoError(t, err)
	err = server.WithMetrics()
	require.NoError(t, err)
	require.NoError(t, server.Start(context.Background()))
	return s, client, server
}
