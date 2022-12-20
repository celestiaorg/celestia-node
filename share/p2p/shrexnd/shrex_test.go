package shrexnd

import (
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

func TestGetSharesWithProofByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net := availability_test.NewTestDAGNet(ctx, t)

	// launch eds store
	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	// create server and register handler
	srv := StartServer(net.NewTestNode().Host, edsStore)
	t.Cleanup(srv.Stop)

	// create client
	client := NewClient(net.NewTestNode().Host, readTimeout)

	net.ConnectAll()

	eds, nID := generateTestEDS(t)
	dah := da.NewDataAvailabilityHeader(eds)

	// put test data into the edsstore and server
	srv.testDAH = &dah
	err = edsStore.Put(ctx, dah.Hash(), eds)
	require.NoError(t, err)

	got, err := client.getBlobByNamespace(
		ctx,
		&dah,
		nID,
		srv.host.ID())

	require.NoError(t, err)
	require.NoError(t, got.Verify(&dah, nID))
}

func newStore(t *testing.T) (*eds.Store, error) {
	t.Helper()

	tmpDir := t.TempDir()
	ds := ds_sync.MutexWrap(datastore.NewMapDatastore())
	return eds.NewStore(tmpDir, ds)
}

func generateTestEDS(t *testing.T) (*rsmt2d.ExtendedDataSquare, namespace.ID) {
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

	bServ := mdutils.Bserv()
	eds, err := share.AddShares(context.Background(), shares, bServ)
	require.NoError(t, err)

	return eds, nID
}
