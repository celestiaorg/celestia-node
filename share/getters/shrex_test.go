package getters

import (
	"context"
	"math/rand"
	"testing"
	"time"

	bsrv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

func TestGetSharesWithProofByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net := availability_test.NewTestDAGNet(ctx, t)

	// generate test data
	bServ := mdutils.Bserv()
	randomEDS, nID := generateTestEDS(t, bServ)
	dah := da.NewDataAvailabilityHeader(randomEDS)

	// launch eds store and put test data into it
	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)
	err = edsStore.Put(ctx, dah.Hash(), randomEDS)
	require.NoError(t, err)

	// create server and register handler
	srvHost := net.NewTestNode().Host
	srv, err := shrexnd.NewServer(srvHost, edsStore, NewIPLDGetter(bServ))
	require.NoError(t, err)
	srv.Start()
	t.Cleanup(srv.Stop)

	// create client and connect it to server
	client, err := shrexnd.NewClient(net.NewTestNode().Host)
	require.NoError(t, err)
	net.ConnectAll()

	got, err := client.RequestND(
		ctx,
		&dah,
		nID,
		srvHost.ID())

	require.NoError(t, err)
	require.NoError(t, got.Verify(&dah, nID))
}

func newStore(t *testing.T) (*eds.Store, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return eds.NewStore(tmpDir, ds)
}

func generateTestEDS(t *testing.T, bServ bsrv.BlockService) (*rsmt2d.ExtendedDataSquare, namespace.ID) {
	shares := share.RandShares(t, 16)

	from := rand.Intn(len(shares))
	to := rand.Intn(len(shares))

	if to < from {
		from, to = to, from
	}

	nID := shares[from][:share.NamespaceSize]
	// change some shares to have same nID
	for i := from; i <= to; i++ {
		copy(shares[i][:share.NamespaceSize], nID)
	}

	eds, err := share.AddShares(context.Background(), shares, bServ)
	require.NoError(t, err)

	return eds, nID
}
