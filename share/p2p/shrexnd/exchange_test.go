package shrexnd

import (
	"context"
	"github.com/celestiaorg/celestia-node/share/store"
	"sync"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/p2p"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestExchange_RequestND_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	edsStore, client, server := makeExchange(t)
	require.NoError(t, server.Start(ctx))

	t.Run("File not exist", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		namespace := sharetest.RandV0Namespace()
		_, err := client.RequestND(ctx, 666, 1, 2, namespace, server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
	})

	t.Run("ErrNamespaceNotFound", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)

		eds := edstest.RandEDS(t, 4)
		dah, err := share.NewRoot(eds)
		height := uint64(42)
		require.NoError(t, err)
		f, err := edsStore.Put(ctx, dah.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		namespace := sharetest.RandV0Namespace()
		fromRow, toRow := share.RowRangeForNamespace(dah, namespace)
		emptyShares, err := client.RequestND(ctx, height, fromRow, toRow, namespace, server.host.ID())
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
		middleware := p2p.NewMiddleware(rateLimit)
		server.host.SetStreamHandler(server.protocolID,
			middleware.RateLimitHandler(mockHandler))

		// take server concurrency slots with blocked requests
		for i := 0; i < rateLimit; i++ {
			go func(i int) {
				client.RequestND(ctx, 1, 1, 2, sharetest.RandV0Namespace(), server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestND(ctx, 1, 1, 2, sharetest.RandV0Namespace(), server.host.ID())
		require.ErrorIs(t, err, p2p.ErrRateLimited)
	})
}

func newStore(t *testing.T) *store.Store {
	t.Helper()

	storeCfg := store.DefaultParameters()
	store, err := store.NewStore(storeCfg, t.TempDir())
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

func makeExchange(t *testing.T) (*store.Store, *Client, *Server) {
	t.Helper()
	store := newStore(t)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultParameters(), hosts[1], store)
	require.NoError(t, err)

	return store, client, server
}
