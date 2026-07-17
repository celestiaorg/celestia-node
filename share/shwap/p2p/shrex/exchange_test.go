package shrex

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

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

// TestClient_AbortsOnCtxCancel is a regression test for #3480: canceling the
// parent context after the stream is opened must abort the request promptly
// instead of blocking on the stream deadline (up to ReadTimeout, 2m default).
func TestClient_AbortsOnCtxCancel(t *testing.T) {
	hosts := createMocknet(t, 2)
	client, err := NewClient(DefaultClientParameters(), hosts[0])
	require.NoError(t, err)

	id, err := shwap.NewNamespaceDataID(1, libshare.RandomNamespace())
	require.NoError(t, err)

	// Server-side handler holds the stream open without reading or responding,
	// simulating an unresponsive peer so the client is stuck in serde.Read.
	serverBlocked := make(chan struct{})
	streamClosed := make(chan struct{})
	hosts[1].SetStreamHandler(
		ProtocolID(client.params.NetworkID(), id.Name()),
		func(s network.Stream) {
			defer close(streamClosed)
			defer s.Close()
			close(serverBlocked)
			// Block until the stream is torn down by the client's Reset.
			buf := make([]byte, 1)
			_, _ = s.Read(buf)
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		result <- client.Get(ctx, &id, &shwap.NamespaceData{}, hosts[1].ID())
	}()

	// Wait until the server has accepted the stream so we know the client is
	// past NewStream and blocked reading the response.
	select {
	case <-serverBlocked:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the stream")
	}

	cancel()

	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("client did not return promptly after context cancellation")
	}
	// The server-side handler should unblock too, once its Read errors out on
	// the reset stream.
	select {
	case <-streamClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("server handler did not exit after client reset the stream")
	}
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
