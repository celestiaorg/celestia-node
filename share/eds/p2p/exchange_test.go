package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	libhost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
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
	eds := share.RandEDS(t, 4)
	dah := da.NewDataAvailabilityHeader(eds)
	err = store.Put(ctx, dah, eds)
	require.NoError(t, err)

	requestedEDS, err := client.RequestEDS(ctx, dah, []peer.ID{server.host.ID()})
	assert.Nil(t, err)
	assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())

	// Testcase: EDS is unavailable initially, but is found after multiple requests
	storageDelay := time.Second * 3
	eds = share.RandEDS(t, 4)
	dah = da.NewDataAvailabilityHeader(eds)
	go func() {
		time.Sleep(storageDelay)
		err = store.Put(ctx, dah, eds)
		// require.NoError(t, err)
	}()

	now := time.Now()
	requestedEDS, err = client.RequestEDS(ctx, dah, []peer.ID{server.host.ID()})
	finished := time.Now()

	assert.Greater(t, finished.Sub(now), storageDelay)
	assert.Nil(t, err)
	assert.Equal(t, eds.Flattened(), requestedEDS.Flattened())
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

	return store, NewClient(hosts[0]), NewServer(hosts[1], store)
}
