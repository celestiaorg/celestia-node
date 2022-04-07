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
	"github.com/libp2p/go-libp2p-core/host"
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

type node struct {
	Service
	format.DAGService
	host.Host
}

type dagNet struct {
	ctx   context.Context
	t     *testing.T
	net   mocknet.Mocknet
	nodes []*node
}

func NewTestDAGNet(ctx context.Context, t *testing.T) *dagNet { //nolint:revive
	return &dagNet{
		ctx: ctx,
		t:   t,
		net: mocknet.New(ctx),
	}
}

func (dn *dagNet) RandLightNode(n int) (*node, *Root) {
	nd := dn.LightNode()
	return nd, RandFillDAG(dn.t, n, nd.DAGService)
}

func (dn *dagNet) RandFullNode(n int) (*node, *Root) {
	nd := dn.FullNode()
	return nd, RandFillDAG(dn.t, n, nd.DAGService)
}

func (dn *dagNet) LightNode() *node {
	nd := dn.Node()
	nd.Service = NewService(nd.DAGService, NewLightAvailability(nd.DAGService))
	return nd
}

func (dn *dagNet) FullNode() *node {
	nd := dn.Node()
	nd.Service = NewService(nd.DAGService, NewFullAvailability(nd.DAGService))
	return nd
}

func (dn *dagNet) Node() *node {
	hst, err := dn.net.GenPeer()
	require.NoError(dn.t, err)
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	bs := bitswap.New(dn.ctx, network.NewFromIpfsHost(hst, routing), bstore, bitswap.ProvideEnabled(false))
	nd := &node{DAGService: merkledag.NewDAGService(blockservice.New(bstore, bs)), Host: hst}
	dn.nodes = append(dn.nodes, nd)
	return nd
}

func (dn *dagNet) ConnectAll() {
	err := dn.net.LinkAll()
	require.NoError(dn.t, err)

	err = dn.net.ConnectAllButSelf()
	require.NoError(dn.t, err)
}

func (dn *dagNet) Connect(peerA, peerB peer.ID) {
	_, err := dn.net.LinkPeers(peerA, peerB)
	require.NoError(dn.t, err)
	_, err = dn.net.ConnectPeers(peerA, peerB)
	require.NoError(dn.t, err)
}

func (dn *dagNet) Disconnect(peerA, peerB peer.ID) {
	err := dn.net.DisconnectPeers(peerA, peerB)
	require.NoError(dn.t, err)
	err = dn.net.UnlinkPeers(peerA, peerB)
	require.NoError(dn.t, err)
}

type subNet struct {
	*dagNet
	nodes []*node
}

func (dn *dagNet) SubNet() *subNet {
	return &subNet{dn, nil}
}

func (sn *subNet) LightNode() *node {
	nd := sn.dagNet.LightNode()
	sn.nodes = append(sn.nodes, nd)
	return nd
}

func (sn *subNet) FullNode() *node {
	nd := sn.dagNet.FullNode()
	sn.nodes = append(sn.nodes, nd)
	return nd
}

func (sn *subNet) ConnectAll() {
	nodes := sn.nodes
	for _, n1 := range nodes {
		for _, n2 := range nodes {
			if n1 == n2 {
				continue
			}
			_, err := sn.net.LinkPeers(n1.ID(), n2.ID())
			require.NoError(sn.t, err)

			_, err = sn.net.ConnectPeers(n1.ID(), n2.ID())
			require.NoError(sn.t, err)
		}
	}
}
