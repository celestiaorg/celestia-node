package share

import (
	"context"
	"testing"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-routing/offline"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/libp2p/go-libp2p-core/peer"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/service/header"
)

// RandLightServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandLightServiceWithSquare(t *testing.T, n int) (Service, *Root) {
	dag := mdutils.Mock()
	return NewService(dag, NewLightAvailability(dag)), RandFillDAG(t, n, dag)
}

// RandFullServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandFullServiceWithSquare(t *testing.T, n int) (Service, *Root) {
	dag := mdutils.Mock()
	return NewService(dag, NewFullAvailability(dag)), RandFillDAG(t, n, dag)
}

func RandFillDAG(t *testing.T, n int, dag format.DAGService) *Root {
	shares := ipld.RandNamespacedShares(t, n*n)
	eds, err := ipld.PutData(context.TODO(), shares.Raw(), dag)
	require.NoError(t, err)
	dah, err := header.DataAvailabilityHeaderFromExtendedData(eds)
	require.NoError(t, err)
	return &dah
}

// RandShares provides 'n' randomized shares prefixed with random namespaces.
func RandShares(t *testing.T, n int) []Share {
	shares := make([]Share, n)
	for i, share := range ipld.RandNamespacedShares(t, n) {
		shares[i] = Share(share.Share)
	}
	return shares
}

type DAGNet struct {
	ctx   context.Context
	t     *testing.T
	net   mocknet.Mocknet
	nodes []peer.ID
}

func NewDAGNet(ctx context.Context, t *testing.T) *DAGNet {
	return &DAGNet{
		ctx: ctx,
		t:   t,
		net: mocknet.New(ctx),
	}
}

func (dn *DAGNet) RandLightService(n int) (Service, *Root) {
	dag, root := dn.RandDAG(n)
	return NewService(dag, NewLightAvailability(dag)), root
}

func (dn *DAGNet) RandFullService(n int) (Service, *Root) {
	dag, root := dn.RandDAG(n)
	return NewService(dag, NewFullAvailability(dag)), root
}

func (dn *DAGNet) RandDAG(n int) (format.DAGService, *Root) {
	dag := dn.CleanDAG()
	return dag, RandFillDAG(dn.t, n, dag)
}

func (dn *DAGNet) CleanLightService() Service {
	dag := dn.CleanDAG()
	return NewService(dag, NewLightAvailability(dag))
}

func (dn *DAGNet) CleanFullService() Service {
	dag := dn.CleanDAG()
	return NewService(dag, NewFullAvailability(dag))
}

func (dn *DAGNet) CleanDAG() format.DAGService {
	nd, err := dn.net.GenPeer()
	require.NoError(dn.t, err)
	dn.nodes = append(dn.nodes, nd.ID())

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	bs := bitswap.New(dn.ctx, network.NewFromIpfsHost(nd, routing), bstore, bitswap.ProvideEnabled(false))
	return merkledag.NewDAGService(blockservice.New(bstore, bs))
}

func (dn *DAGNet) ConnectAll() {
	err := dn.net.LinkAll()
	require.NoError(dn.t, err)

	err = dn.net.ConnectAllButSelf()
	require.NoError(dn.t, err)
}

func (dn *DAGNet) Disconnect(peerA, peerB int) {
	err := dn.net.DisconnectPeers(dn.nodes[peerA], dn.nodes[peerB])
	require.NoError(dn.t, err)
}
