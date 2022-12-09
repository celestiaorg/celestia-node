package shrex

import (
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
	"golang.org/x/net/context"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/p2p"
	"github.com/celestiaorg/nmt/namespace"
)

func TestGetSharesWithProofByNamespace(t *testing.T) {
	net := mocknet.New()
	srvHost, err := net.GenPeer()
	require.NoError(t, err)

	defaultTimeout := time.Second * 10
	sCfg := p2p.Config{
		WriteTimeout: defaultTimeout,
		ReadTimeout:  defaultTimeout,
		Interceptors: nil,
	}
	server := p2p.NewServer(sCfg, srvHost)

	availSvc := &availStub{}
	availability := NewAvailabilityHandler(availSvc, getterStub{})
	server.RegisterHandler(ndProtocolID, availability.Handle)
	defer server.RemoveHandler(ndProtocolID)

	clHost, err := net.GenPeer()
	require.NoError(t, err)

	cCfg := p2p.ClientConfig{
		ReadTimeout:  defaultTimeout,
		WriteTimeout: defaultTimeout,
		Interceptors: nil,
	}
	client := Client{client: p2p.NewCLient(cCfg, clHost)}

	err = net.LinkAll()
	require.NoError(t, err)

	type args struct {
		ctx           context.Context
		height        uint64
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
			"get by height",
			client,
			args{
				ctx:           context.TODO(),
				height:        1,
				namespace:     make([]byte, 8),
				collectProofs: true,
				peerID:        srvHost.ID(),
			},
			randomSharesWithProof(t, true),
			false,
		},
		{
			"get by head",
			client,
			args{
				ctx:           context.TODO(),
				height:        0,
				namespace:     make([]byte, 8),
				collectProofs: true,
				peerID:        srvHost.ID(),
			},
			randomSharesWithProof(t, true),
			false,
		},
		{
			"no proofs, thanks",
			client,
			args{
				ctx:           context.TODO(),
				height:        0,
				namespace:     make([]byte, 8),
				collectProofs: false,
				peerID:        srvHost.ID(),
			},
			randomSharesWithProof(t, false),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			availSvc.with(tt.want)
			got, err := tt.client.GetSharesWithProofs(
				tt.args.ctx, tt.args.height, tt.args.namespace, tt.args.collectProofs, tt.args.peerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSharesWithProofs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

type getterStub struct{}

func (m getterStub) IsSyncing(ctx context.Context) bool {
	panic("implement me")
}

func (m getterStub) Head(context.Context) (*header.ExtendedHeader, error) {
	return &header.ExtendedHeader{
		Commit:    &types.Commit{},
		RawHeader: header.RawHeader{Height: int64(42)},
		DAH:       &header.DataAvailabilityHeader{RowsRoots: make([][]byte, 0)}}, nil
}

func (m getterStub) GetByHeight(_ context.Context, height uint64) (*header.ExtendedHeader, error) {
	return &header.ExtendedHeader{
		Commit:    &types.Commit{},
		RawHeader: header.RawHeader{Height: int64(height)},
		DAH:       &header.DataAvailabilityHeader{RowsRoots: make([][]byte, 0)}}, nil
}

func (m getterStub) GetRangeByHeight(ctx context.Context, from, amount uint64) ([]*header.ExtendedHeader, error) {
	return nil, nil
}

func (m getterStub) GetVerifiedRange(
	context.Context,
	*header.ExtendedHeader,
	uint64,
) ([]*header.ExtendedHeader, error) {
	return nil, nil
}

func (m getterStub) Get(context.Context, tmbytes.HexBytes) (*header.ExtendedHeader, error) {
	return nil, nil
}

type availStub struct {
	sharesWithProofs []share.SharesWithProof
}

func (a *availStub) with(swp []share.SharesWithProof) {
	a.sharesWithProofs = swp
}

func (a *availStub) ProbabilityOfAvailability(ctx context.Context) float64 {
	panic("implement me")
}

func (a *availStub) SharesAvailable(ctx context.Context, root *share.Root) error {
	panic("implement me")
}

func (a *availStub) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	panic("implement me")
}

func (a *availStub) GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error) {
	panic("implement me")
}

func (a *availStub) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	namespace namespace.ID,
) ([]share.Share, error) {
	panic("implement me")
}

func (a *availStub) GetSharesWithProofsByNamespace(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
) ([]share.SharesWithProof, error) {
	return a.sharesWithProofs, nil
}

func randomSharesWithProof(t *testing.T, withProof bool) []share.SharesWithProof {
	shares := share.RandShares(t, 16)
	// set all shares to the same namespace and data but the last one
	nid := shares[0][:ipld.NamespaceSize]

	for _, sh := range shares {
		copy(sh[:ipld.NamespaceSize], nid)
	}

	proof := ipld.Proof{
		Nodes: []cid.Cid{},
	}
	if withProof {
		proof.Start = 1
		proof.End = 2
	}

	return []share.SharesWithProof{
		{
			Shares: shares,
			Proof:  &proof,
		},
	}
}
