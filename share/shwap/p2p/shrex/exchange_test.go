package shrex

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"

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
