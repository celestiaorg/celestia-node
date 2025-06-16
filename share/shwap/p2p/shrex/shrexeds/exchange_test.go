package shrexeds

import (
	"context"
	"testing"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
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
