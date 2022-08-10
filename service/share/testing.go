package share

import (
	"bytes"
	"context"
	"testing"
	"time"

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
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld"
)

// RandLightServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandLightServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	return NewService(bServ, TestLightAvailability(bServ)), RandFillBS(t, n, bServ)
}

// RandLightService provides an unfilled share.Service with corresponding
// blockservice.BlockService than can be filled by the test.
func RandLightService() (*Service, blockservice.BlockService) {
	bServ := mdutils.Bserv()
	return NewService(bServ, TestLightAvailability(bServ)), bServ
}

// RandFullServiceWithSquare provides a share.Service filled with 'n' NMT
// trees of 'n' random shares, essentially storing a whole square.
func RandFullServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	return NewService(bServ, TestFullAvailability(bServ)), RandFillBS(t, n, bServ)
}

// RandLightLocalServiceWithSquare is the same as RandLightServiceWithSquare, except
// the Availability is wrapped with CacheAvailability.
func RandLightLocalServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	ds := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewCacheAvailability(
		TestLightAvailability(bServ),
		ds,
	)
	return NewService(bServ, ca), RandFillBS(t, n, bServ)
}

// RandFullLocalServiceWithSquare is the same as RandFullServiceWithSquare, except
// the Availability is wrapped with CacheAvailability.
func RandFullLocalServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	ds := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewCacheAvailability(
		TestFullAvailability(bServ),
		ds,
	)
	return NewService(bServ, ca), RandFillBS(t, n, bServ)
}

// RandFillBS fills the given BlockService with a random block of a given size.
func RandFillBS(t *testing.T, n int, bServ blockservice.BlockService) *Root {
	shares := RandShares(t, n*n)
	return FillBS(t, bServ, shares)
}

// FillBS fills the given BlockService with the given shares.
func FillBS(t *testing.T, bServ blockservice.BlockService, shares []Share) *Root {
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
	net *dagNet
	*Service
	blockservice.BlockService
	host.Host
}

// ClearStorage cleans up the storage of the node.
func (n *node) ClearStorage() {
	keys, err := n.Blockstore().AllKeysChan(n.net.ctx)
	require.NoError(n.net.t, err)

	for k := range keys {
		err := n.DeleteBlock(n.net.ctx, k)
		require.NoError(n.net.t, err)
	}
}

type dagNet struct {
	ctx   context.Context
	t     *testing.T
	net   mocknet.Mocknet
	nodes []*node
}

// NewTestDAGNet creates a new testing swarm utility to spawn different nodes
// and test how they interact and/or exchange data.
func NewTestDAGNet(ctx context.Context, t *testing.T) *dagNet { //nolint:revive
	return &dagNet{
		ctx: ctx,
		t:   t,
		net: mocknet.New(),
	}
}

// RandLightNode creates a Light Node filled with a random block of the given size.
func (dn *dagNet) RandLightNode(squareSize int) (*node, *Root) {
	nd := dn.LightNode()
	return nd, RandFillBS(dn.t, squareSize, nd.BlockService)
}

// RandFullNode creates a Full Node filled with a random block of the given size.
func (dn *dagNet) RandFullNode(squareSize int) (*node, *Root) {
	nd := dn.FullNode()
	return nd, RandFillBS(dn.t, squareSize, nd.BlockService)
}

// LightNode creates a new empty LightAvailability Node.
func (dn *dagNet) LightNode() *node {
	nd := dn.Node()
	nd.Service = NewService(nd.BlockService, TestLightAvailability(nd.BlockService))
	return nd
}

// FullNode creates a new empty FullAvailability Node.
func (dn *dagNet) FullNode() *node {
	nd := dn.Node()
	nd.Service = NewService(nd.BlockService, TestFullAvailability(nd.BlockService))
	return nd
}

// Node create a plain network node that can serve and request data.
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
	nd := &node{
		net:          dn,
		BlockService: blockservice.New(bstore, bs),
		Host:         hst,
	}
	dn.nodes = append(dn.nodes, nd)
	return nd
}

// ConnectAll connects all the peers on registered on the dagNet.
func (dn *dagNet) ConnectAll() {
	err := dn.net.LinkAll()
	require.NoError(dn.t, err)

	err = dn.net.ConnectAllButSelf()
	require.NoError(dn.t, err)
}

// Connect connects two given peer.
func (dn *dagNet) Connect(peerA, peerB peer.ID) {
	_, err := dn.net.LinkPeers(peerA, peerB)
	require.NoError(dn.t, err)
	_, err = dn.net.ConnectPeers(peerA, peerB)
	require.NoError(dn.t, err)
}

// Disconnect disconnects two peers.
// It does a hard disconnect, meaning that disconnected peers won't be able to reconnect on their own
// but only with dagNet.Connect or dagNet.ConnectAll.
func (dn *dagNet) Disconnect(peerA, peerB peer.ID) {
	err := dn.net.UnlinkPeers(peerA, peerB)
	require.NoError(dn.t, err)
	err = dn.net.DisconnectPeers(peerA, peerB)
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

type TestBrokenAvailability struct {
	Root *Root
}

// NewTestBrokenAvailability returns an instance of Availability that
// allows for testing error cases during sampling.
//
// If the Root field is empty, it will return ErrNotAvailable on every call
// to SharesAvailable. Otherwise, it will only return ErrNotAvailable if the
// given Root hash matches the stored Root hash.
func NewTestBrokenAvailability() Availability {
	return &TestBrokenAvailability{}
}

func (b *TestBrokenAvailability) SharesAvailable(_ context.Context, root *Root) error {
	if b.Root == nil || bytes.Equal(b.Root.Hash(), root.Hash()) {
		return ErrNotAvailable
	}
	return nil
}

func (b *TestBrokenAvailability) ProbabilityOfAvailability() float64 {
	return 0
}

func TestLightAvailability(bServ blockservice.BlockService) *LightAvailability {
	disc := NewDiscovery(nil, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	return NewLightAvailability(bServ, disc)
}

func TestFullAvailability(bServ blockservice.BlockService) *FullAvailability {
	disc := NewDiscovery(nil, routing.NewRoutingDiscovery(routinghelpers.Null{}), 0, time.Second, time.Second)
	return NewFullAvailability(bServ, disc)
}

type TestSuccessfulAvailability struct {
}

// NewTestSuccessfulAvailability returns an Availability that always
// returns successfully when SharesAvailable is called.
func NewTestSuccessfulAvailability() Availability {
	return &TestSuccessfulAvailability{}
}

func (tsa *TestSuccessfulAvailability) SharesAvailable(context.Context, *Root) error {
	return nil
}

func (tsa *TestSuccessfulAvailability) ProbabilityOfAvailability() float64 {
	return 0
}
