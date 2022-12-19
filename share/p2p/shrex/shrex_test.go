package shrex

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
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

func TestGetSharesWithProofByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net := availability_test.NewTestDAGNet(ctx, t)

	edsStore, err := newStore(t)
	require.NoError(t, err)
	err = edsStore.Start(ctx)
	require.NoError(t, err)

	// create server and register handler
	getter := &mockGetter{}
	srv := StartServer(net.NewTestNode().Host, edsStore)
	srv.getter = getter
	t.Cleanup(srv.Stop)

	// create client
	client := NewClient(net.NewTestNode().Host, readTimeout)

	net.ConnectAll()

	// generate test data
	blob, eds, nID := generateTest(t)
	dah := da.NewDataAvailabilityHeader(eds)

	// put data into the store and getter mock
	getter.blob = blob
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

func generateTest(t *testing.T) (*share.Blob, *rsmt2d.ExtendedDataSquare, namespace.ID) {
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

	var rows []share.VerifiedShares
	for _, row := range eds.RowRoots() {
		rcid := ipld.MustCidFromNamespacedSha256(row)

		proof := new(ipld.Proof)
		rowShares, err := share.GetSharesByNamespace(context.Background(), bServ, rcid, nID, len(eds.RowRoots()), proof)
		require.NoError(t, err)

		if len(rowShares) != 0 {
			rows = append(rows, share.VerifiedShares{
				Shares: rowShares,
				Proof:  proof,
			})
		}
	}

	return &share.Blob{Rows: rows}, eds, nID
}

type mockGetter struct {
	blob *share.Blob
}

func (m *mockGetter) GetBlobByNamespace(context.Context, *share.Root, namespace.ID) (*share.Blob, error) {
	return m.blob, nil
}
