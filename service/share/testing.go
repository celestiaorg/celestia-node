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
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld"
)

// RandLightServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandLightServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	return NewService(bServ, NewLightAvailability(bServ)), RandFillDAG(t, n, bServ)
}

// RandLightService provides an unfilled share.Service with corresponding
// blockservice.BlockService than can be filled by the test.
func RandLightService() (*Service, blockservice.BlockService) {
	bServ := mdutils.Bserv()
	return NewService(bServ, NewLightAvailability(bServ)), bServ
}

// RandFullServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandFullServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	return NewService(bServ, NewFullAvailability(bServ)), RandFillDAG(t, n, bServ)
}

// RandLightLocalServiceWithSquare is the same as RandLightServiceWithSquare, except
// the Availability is wrapped with CacheAvailability.
func RandLightLocalServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	ds := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewCacheAvailability(NewLightAvailability(bServ), ds)
	return NewService(bServ, ca), RandFillDAG(t, n, bServ)
}

// RandFullLocalServiceWithSquare is the same as RandFullServiceWithSquare, except
// the Availability is wrapped with CacheAvailability.
func RandFullLocalServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	ds := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewCacheAvailability(NewFullAvailability(bServ), ds)
	return NewService(bServ, ca), RandFillDAG(t, n, bServ)
}

func RandFillDAG(t *testing.T, n int, bServ blockservice.BlockService) *Root {
	shares := RandShares(t, n*n)
	return FillDag(t, bServ, shares)
}

func FillDag(t *testing.T, bServ blockservice.BlockService, shares []Share) *Root {
	eds, err := ipld.AddShares(context.TODO(), shares, bServ)
	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(eds)
	return &dah
}

// RandShares provides 'n' randomized shares prefixed with random namespaces.
func RandShares(t *testing.T, n int) []Share {
	return ipld.RandShares(t, n)
}

type node struct {
	*Service
	blockservice.BlockService
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
	return nd, RandFillDAG(dn.t, n, nd.BlockService)
}

func (dn *dagNet) RandFullNode(n int) (*node, *Root) {
	nd := dn.FullNode()
	return nd, RandFillDAG(dn.t, n, nd.BlockService)
}

func (dn *dagNet) LightNode() *node {
	nd := dn.Node()
	nd.Service = NewService(nd.BlockService, NewLightAvailability(nd.BlockService))
	return nd
}

func (dn *dagNet) FullNode() *node {
	nd := dn.Node()
	nd.Service = NewService(nd.BlockService, NewFullAvailability(nd.BlockService))
	return nd
}

func (dn *dagNet) Node() *node {
	hst, err := dn.net.GenPeer()
	require.NoError(dn.t, err)
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	bs := bitswap.New(
		dn.ctx,
		network.NewFromIpfsHost(hst, routing),
		bstore,
		bitswap.ProvideEnabled(false),          // disable routines for DHT content provides, as we don't use them
		bitswap.EngineBlockstoreWorkerCount(1), // otherwise it spawns 128 routines which is too much for tests
		bitswap.EngineTaskWorkerCount(2),
		bitswap.TaskWorkerCount(2),
		bitswap.SetSimulateDontHavesOnTimeout(false),
		bitswap.SetSendDontHaves(false),
	)
	nd := &node{BlockService: blockservice.New(bstore, bs), Host: hst}
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

// brokenAvailability allows to test error cases during sampling
type brokenAvailability struct{}

func NewBrokenAvailability() Availability {
	return &brokenAvailability{}
}

func (b *brokenAvailability) SharesAvailable(context.Context, *Root) error {
	return ErrNotAvailable
}
