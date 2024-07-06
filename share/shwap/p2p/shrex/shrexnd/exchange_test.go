package shrexnd

import (
	"context"
	"sync"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/store"
)

func TestExchange_RequestND_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	edsStore, client, server := makeExchange(t)
	require.NoError(t, server.Start(ctx))

	height := atomic.NewUint64(1)

	t.Run("CAR_not_exist", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		namespace := sharetest.RandV0Namespace()
		height := height.Add(1)
		_, err := client.RequestND(ctx, height, namespace, server.host.ID())
		require.ErrorIs(t, err, shrex.ErrNotFound)
	})

	t.Run("ErrNamespaceNotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)

		height := height.Add(1)
		err = edsStore.Put(ctx, roots, height, eds)
		require.NoError(t, err)

		namespace := sharetest.RandV0Namespace()
		emptyShares, err := client.RequestND(ctx, height, namespace, server.host.ID())
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
		server, err := NewServer(DefaultParameters(), net.Hosts()[1], nil)
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
		middleware := shrex.NewMiddleware(rateLimit)
		server.host.SetStreamHandler(server.protocolID,
			middleware.RateLimitHandler(mockHandler))

		// take server concurrency slots with blocked requests
		for i := 0; i < rateLimit; i++ {
			go func(i int) {
				client.RequestND(ctx, 1, sharetest.RandV0Namespace(), server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestND(ctx, 1, sharetest.RandV0Namespace(), server.host.ID())
		require.ErrorIs(t, err, shrex.ErrRateLimited)
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

	client, err := NewClient(DefaultParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultParameters(), hosts[1], s)
	require.NoError(t, err)

	return s, client, server
}
