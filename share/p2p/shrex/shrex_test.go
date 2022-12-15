package shrex

import (
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

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
	srvNode := net.NewTestNode()
	getter := &mockGetter{}
	availability := NewAvailabilityHandler(getter)
	srvNode.Host.SetStreamHandler(ndProtocolID, availability.Handle)
	defer srvNode.Host.RemoveStreamHandler(ndProtocolID)

	// create client
	clNode := net.NewTestNode()
	client := Client{host: clNode.Host, defaultTimeout: readTimeout}

	net.ConnectAll()

	type args struct {
		ctx           context.Context
		rootHash      []byte
		rowRoots      [][]byte
		maxShares     int
		namespace     namespace.ID
		collectProofs bool
		peerID        peer.ID
	}

	tests := []struct {
		name    string
		client  Client
		args    args
		want    []share.SharesWithProof
		wantErr bool
	}{
		{
			"with proofs",
			client,
			args{
				ctx:           context.TODO(),
				rootHash:      []byte{},
				rowRoots:      [][]byte{make([]byte, share.NamespaceSize*2)},
				namespace:     make([]byte, share.NamespaceSize),
				collectProofs: true,
				peerID:        srvNode.ID(),
			},
			randomSharesWithProof(t, true),
			false,
		},
		{
			"no proofs, thanks",
			client,
			args{
				ctx:           context.TODO(),
				rootHash:      []byte{},
				rowRoots:      [][]byte{make([]byte, share.NamespaceSize*2)},
				namespace:     make([]byte, share.NamespaceSize),
				collectProofs: false,
				peerID:        srvNode.ID(),
			},
			randomSharesWithProof(t, false),
			false,
		},
		{
			"incorrect namespaceID",
			client,
			args{
				ctx:           context.TODO(),
				rootHash:      []byte{},
				rowRoots:      [][]byte{make([]byte, share.NamespaceSize*2)},
				namespace:     nil,
				collectProofs: false,
				peerID:        srvNode.ID(),
			},
			randomSharesWithProof(t, false),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter.sharesWithProof = tt.want
			got, err := tt.client.GetSharesWithProofs(
				tt.args.ctx,
				tt.args.rootHash,
				tt.args.rowRoots,
				tt.args.maxShares,
				tt.args.namespace,
				tt.args.collectProofs,
				tt.args.peerID)
			if err != nil {
				if tt.wantErr {
					return
				}
				t.Errorf("GetSharesWithProofs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, len(tt.want), len(got))
			for i := range tt.want {
				assert.Equal(t, tt.want[i].Shares, got[i].Shares)
				assert.Equal(t, tt.want[i].Proof, got[i].Proof)
			}
		})
	}
}

func randomSharesWithProof(t *testing.T, withProof bool) []share.SharesWithProof {
	shares := share.RandShares(t, 16)
	// set all shares to the same namespace and data but the last one
	nid := shares[0][:ipld.NamespaceSize]

	for _, sh := range shares {
		copy(sh[:ipld.NamespaceSize], nid)
	}

	var proof *ipld.Proof

	if withProof {
		proof = &ipld.Proof{
			Nodes: []cid.Cid{},
			Start: 1,
			End:   2,
		}
	}

	return []share.SharesWithProof{
		{
			Shares: shares,
			Proof:  proof,
		},
	}
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
