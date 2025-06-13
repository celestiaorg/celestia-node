package shrexeds

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/store"
)

func TestExchange_RequestEDS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	store, client, server := makeExchange(t)

	err := server.Start(ctx)
	require.NoError(t, err)

	// Testcase: EDS is immediately available
	t.Run("EDS_Available", func(t *testing.T) {
		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		height := uint64(1)
		err = store.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)

		requestedEDS, err := client.RequestEDS(ctx, roots, height, server.host.ID())
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
	})

	// Testcase: EDS is unavailable initially, but is found after multiple requests
	t.Run("EDS_AvailableAfterDelay", func(t *testing.T) {
		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		height := uint64(666)

		lock := make(chan struct{})
		go func() {
			<-lock
			err := store.PutODSQ4(ctx, roots, height, eds)
			require.NoError(t, err)
			lock <- struct{}{}
		}()

		requestedEDS, err := client.RequestEDS(ctx, roots, height, server.host.ID())
		assert.ErrorIs(t, err, shrex.ErrNotFound)
		assert.Nil(t, requestedEDS)

		// unlock write
		lock <- struct{}{}
		// wait for write to finish
		<-lock

		requestedEDS, err = client.RequestEDS(ctx, roots, height, server.host.ID())
		assert.NoError(t, err)
		assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
	})

	// Testcase: Invalid request excludes peer from round-robin, stopping request
	t.Run("EDS_InvalidRequest", func(t *testing.T) {
		emptyRoot := share.EmptyEDSRoots()
		height := uint64(0)
		requestedEDS, err := client.RequestEDS(ctx, emptyRoot, height, server.host.ID())
		assert.ErrorIs(t, err, shwap.ErrInvalidID)
		assert.Nil(t, requestedEDS)
	})

	t.Run("EDS_err_not_found", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
		t.Cleanup(cancel)
		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		height := uint64(668)
		_, err = client.RequestEDS(timeoutCtx, roots, height, server.host.ID())
		require.ErrorIs(t, err, shrex.ErrNotFound)
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
		middleware := shrex.NewMiddleware(rateLimit)
		server.host.SetStreamHandler(server.protocolID,
			middleware.RateLimitHandler(mockHandler))

		// take server concurrency slots with blocked requests
		emptyRoot := share.EmptyEDSRoots()
		for i := range rateLimit {
			go func(i int) {
				client.RequestEDS(ctx, emptyRoot, 1, server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestEDS(ctx, emptyRoot, 1, server.host.ID())
		require.ErrorIs(t, err, shrex.ErrNotFound)
	})

	// Testcase: Context cancellation should return quickly
	t.Run("EDS_ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		eds := edstest.RandEDS(t, 4)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		height := uint64(2)

		// create a slow handler that simulates a slow server response
		slowHandler := func(stream network.Stream) {
			time.Sleep(100 * time.Millisecond)

			select {
			case <-ctx.Done():
				stream.Reset() //nolint:errcheck
				return
			default:
			}

			time.Sleep(2 * time.Second)
		}

		// set slow handler
		server.host.SetStreamHandler(server.protocolID, slowHandler)

		// cancel after short deplay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		_, err = client.RequestEDS(ctx, roots, height, server.host.ID())
		elapsed := time.Since(start)

		assert.Error(t, err)
isCanceled := errors.Is(err, context.Canceled)
	isStreamReset := err != nil && strings.Contains(err.Error(), "stream reset")
	isCanceledOrReset := isCanceled || isStreamReset
		assert.True(t, isCanceledOrReset, "Expected context cancellation error, got: %v", err)
		assert.Less(t, elapsed, 500*time.Millisecond, "Request should return quickly on context cancellation")
	})

	// Testcase: Context cancellation during data reading should abort quickly
	t.Run("EDS_ContextCancellationDuringDataReading", func(t *testing.T) {
		store, client, server := makeExchange(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := server.Start(ctx)
		require.NoError(t, err)

		eds := edstest.RandEDS(t, 32)
		roots, err := share.NewAxisRoots(eds)
		require.NoError(t, err)
		height := uint64(3)

		err = store.PutODSQ4(ctx, roots, height, eds)
		require.NoError(t, err)

		// cancel the context immediately to simulate a quick cancellation
		reqCtx, reqCancel := context.WithCancel(context.Background())
		reqCancel()

		_, err = client.RequestEDS(reqCtx, roots, height, server.host.ID())
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled), "Expected context.Canceled error, got: %v", err)
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
	store, err := store.NewStore(store.DefaultParameters(), t.TempDir())
	require.NoError(t, err)
	hosts := createMocknet(t, 2)

	client, err := NewClient(DefaultParameters(), hosts[0])
	require.NoError(t, err)
	server, err := NewServer(DefaultParameters(), hosts[1], store)
	require.NoError(t, err)

	return store, client, server
}
