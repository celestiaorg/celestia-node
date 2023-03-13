package shrexnd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/p2p"
)

func TestExchange_RequestND(t *testing.T) {
	// Testcase: Concurrency limit reached
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
		server.host.SetStreamHandler(server.protocolID,
			p2p.RateLimitMiddleware(mockHandler, rateLimit))

		// take server concurrency slots with blocked requests
		for i := 0; i < rateLimit; i++ {
			go func(i int) {
				client.RequestND(ctx, nil, nil, server.host.ID()) //nolint:errcheck
			}(i)
		}

		// wait until all server slots are taken
		wg.Wait()
		_, err = client.RequestND(ctx, nil, nil, server.host.ID())
		require.ErrorIs(t, err, p2p.ErrUnavailable)
	})
}
