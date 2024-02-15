package shrexeds

import (
	"context"
	"sync"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/p2p"
	"github.com/celestiaorg/celestia-node/share/store"
)

func TestExchange_RequestEDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	store, client, server := makeExchange(t)
	err := server.Start(ctx)
	require.NoError(t, err)

	height := atomic.NewUint64(1)

	// Testcase: EDS is immediately available
	t.Run("EDS_Available", func(t *testing.T) {
		eds, root := testData(t)
		height := height.Add(1)
		f, err := store.Put(ctx, root.Hash(), height, eds)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		requestedEDS, err := client.RequestEDS(ctx, root, height, server.host.ID())
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
	})

	// Testcase: EDS is unavailable initially, but is found after multiple requests
	t.Run("EDS_AvailableAfterDelay", func(t *testing.T) {
		eds, root := testData(t)
		height := height.Add(1)

		lock := make(chan struct{})
		go func() {
			<-lock
			f, err := store.Put(ctx, root.Hash(), height, eds)
			require.NoError(t, err)
			require.NoError(t, f.Close())
			require.NoError(t, err)
			lock <- struct{}{}
		}()

		requestedEDS, err := client.RequestEDS(ctx, root, height, server.host.ID())
		assert.ErrorIs(t, err, p2p.ErrNotFound)
		assert.Nil(t, requestedEDS)

		// unlock write
		lock <- struct{}{}
		// wait for write to finish
		<-lock

		requestedEDS, err = client.RequestEDS(ctx, root, height, server.host.ID())
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
	})

	t.Run("EDS_err_not_found", func(t *testing.T) {
		_, root := testData(t)
		height := height.Add(1)
		require.NoError(t, err)
		_, err = client.RequestEDS(ctx, root, height, server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
	})

	// Testcase: Concurrency limit reached
	t.Run("EDS_concurrency_limit", func(t *testing.T) {
		_, client, server := makeExchange(t)
		require.NoError(t, server.Start(ctx))

		ctx, cancel := context.WithTimeout(ctx, time.Second)
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
		height := height.Add(1)
		for i := 0; i < rateLimit; i++ {
			go func(i int) {
				client.RequestEDS(ctx, nil, height, server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestEDS(ctx, nil, height, server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
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
	cfg := store.DefaultParameters()
	store, err := store.NewStore(cfg, t.TempDir())
	require.NoError(t, err)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultParameters(), hosts[1], store)
	require.NoError(t, err)

	return store, client, server
}

func testData(t *testing.T) (*rsmt2d.ExtendedDataSquare, *share.Root) {
	eds := edstest.RandEDS(t, 4)
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	return eds, dah
}
