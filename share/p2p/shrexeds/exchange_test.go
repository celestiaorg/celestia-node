package shrexeds

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/p2p"
)

func TestExchange_RequestEDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	store, client, server := makeExchange(t)

	err := store.Start(ctx)
	require.NoError(t, err)

	err = server.Start(ctx)
	require.NoError(t, err)

	// Testcase: EDS is immediately available
	t.Run("EDS_Available", func(t *testing.T) {
		eds := edstest.RandEDS(t, 4)
		dah := da.NewDataAvailabilityHeader(eds)
		err = store.Put(ctx, dah.Hash(), eds)
		require.NoError(t, err)

		requestedEDS, err := client.RequestEDS(ctx, dah.Hash(), server.host.ID())
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
	})

	// Testcase: EDS is unavailable initially, but is found after multiple requests
	t.Run("EDS_AvailableAfterDelay", func(t *testing.T) {
		storageDelay := time.Second
		eds := edstest.RandEDS(t, 4)
		dah := da.NewDataAvailabilityHeader(eds)
		go func() {
			time.Sleep(storageDelay)
			err = store.Put(ctx, dah.Hash(), eds)
			// require.NoError(t, err)
		}()

		requestedEDS, err := client.RequestEDS(ctx, dah.Hash(), server.host.ID())
		assert.ErrorIs(t, err, p2p.ErrNotFound)
		assert.Nil(t, requestedEDS)

		time.Sleep(storageDelay * 2)
		requestedEDS, err = client.RequestEDS(ctx, dah.Hash(), server.host.ID())
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
	})

	// Testcase: Invalid request excludes peer from round-robin, stopping request
	t.Run("EDS_InvalidRequest", func(t *testing.T) {
		dataHash := []byte("invalid")
		requestedEDS, err := client.RequestEDS(ctx, dataHash, server.host.ID())
		assert.ErrorContains(t, err, "stream reset")
		assert.Nil(t, requestedEDS)
	})

	t.Run("EDS_err_not_found", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)
		eds := edstest.RandEDS(t, 4)
		dah := da.NewDataAvailabilityHeader(eds)
		_, err := client.RequestEDS(timeoutCtx, dah.Hash(), server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
	})

	// Testcase: Concurrency limit reached
	t.Run("EDS_concurrency_limit", func(t *testing.T) {
		store, client, server := makeExchange(t)

		require.NoError(t, store.Start(ctx))
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
		for i := 0; i < rateLimit; i++ {
			go func(i int) {
				client.RequestEDS(ctx, nil, server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestEDS(ctx, nil, server.host.ID())
		require.ErrorIs(t, err, p2p.ErrNotFound)
	})
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

func makeExchange(t *testing.T) (*eds.Store, *Client, *Server) {
	t.Helper()
	store := newStore(t)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultParameters(), hosts[1], store)
	require.NoError(t, err)

	return store, client, server
}
