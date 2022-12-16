package shrex

import (
	"math/rand"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/share"
	availability_test "github.com/celestiaorg/celestia-node/share/availability/test"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt/namespace"
)

func TestGetSharesWithProofByNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	// create test net
	net := availability_test.NewTestDAGNet(ctx, t)

	// create server and register handler
	getter := &mockGetter{}
	srv := StartServer(net.NewTestNode().Host, getter)
	t.Cleanup(srv.Stop)

	// create client
	client := NewClient(net.NewTestNode().Host, readTimeout)

	net.ConnectAll()

	t.Run("with proofs", func(t *testing.T) {
		shares, dah, nID := randomSharesWithProof(t, true)

		getter.sharesWithProof = shares
		got, err := client.getSharesWithProofs(
			ctx,
			dah.Hash(),
			dah.RowsRoots,
			nID,
			true,
			srv.host.ID())

		require.NoError(t, err)

		require.Equal(t, len(shares), len(got))
		for i := range shares {
			assert.Equal(t, shares[i].Shares, got[i].Shares)
			assert.Equal(t, shares[i].Proof, got[i].Proof)
		}
	})

	t.Run("without proofs", func(t *testing.T) {
		shares, dah, nID := randomSharesWithProof(t, false)

		getter.sharesWithProof = shares
		got, err := client.getSharesWithProofs(
			ctx,
			dah.Hash(),
			dah.RowsRoots,
			nID,
			false,
			srv.host.ID())

		require.NoError(t, err)

		require.Equal(t, len(shares), len(got))
		for i := range shares {
			assert.Equal(t, shares[i].Shares, got[i].Shares)
			assert.Nil(t, got[i].Proof)
		}
	})
}

func randomSharesWithProof(t *testing.T, collectProof bool,
) ([]share.SharesWithProof, da.DataAvailabilityHeader, namespace.ID) {
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

	var sharesWithProof []share.SharesWithProof
	for _, row := range eds.RowRoots() {
		rcid := ipld.MustCidFromNamespacedSha256(row)

		var proof *ipld.Proof
		if collectProof {
			proof = new(ipld.Proof)
		}
		rowShares, err := share.GetSharesByNamespace(context.Background(), bServ, rcid, nID, len(eds.RowRoots()), proof)
		require.NoError(t, err)

		if len(rowShares) != 0 {
			sharesWithProof = append(sharesWithProof, share.SharesWithProof{
				Shares: rowShares,
				Proof:  proof,
			})
		}
	}

	dah := da.NewDataAvailabilityHeader(eds)
	return sharesWithProof, dah, nID
}

type mockGetter struct {
	sharesWithProof []share.SharesWithProof
}

func (m *mockGetter) GetSharesWithProofsByNamespace(
	ctx context.Context,
	rootHash []byte,
	rowRoots [][]byte,
	maxShares int,
	nID namespace.ID,
	collectProofs bool,
) ([]share.SharesWithProof, error) {
	return m.sharesWithProof, nil
}
