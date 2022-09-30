package availability_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-routing/offline"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/pkg/da"
)

// RandFillBS fills the given BlockService with a random block of a given size.
func RandFillBS(t *testing.T, n int, bServ blockservice.BlockService) *share.Root {
	shares := RandShares(t, n*n)
	return FillBS(t, bServ, shares)
}

// FillBS fills the given BlockService with the given shares.
func FillBS(t *testing.T, bServ blockservice.BlockService, shares []share.Share) *share.Root {
	eds, err := share.AddShares(context.TODO(), shares, bServ)
	require.NoError(t, err)
	dah := da.NewDataAvailabilityHeader(eds)
	return &dah
}

// RandShares provides 'n' randomized shares prefixed with random namespaces.
func RandShares(t *testing.T, n int) []share.Share {
	return share.RandShares(t, n)
}

type Node struct {
	net *DagNet
	*availability.Service
	blockservice.BlockService
	host.Host
}

// ClearStorage cleans up the storage of the node.
func (n *Node) ClearStorage() {
	keys, err := n.Blockstore().AllKeysChan(n.net.ctx)
	require.NoError(n.net.T, err)

	for k := range keys {
		err := n.DeleteBlock(n.net.ctx, k)
		require.NoError(n.net.T, err)
	}
}

type DagNet struct {
	ctx   context.Context
	T     *testing.T
	net   mocknet.Mocknet
	nodes []*Node
}

// NewTestDAGNet creates a new testing swarm utility to spawn different nodes
// and test how they interact and/or exchange data.
func NewTestDAGNet(ctx context.Context, t *testing.T) *DagNet {
	return &DagNet{
		ctx: ctx,
		T:   t,
		net: mocknet.New(),
	}
}

// Node create a plain network node that can serve and request data.
func (dn *DagNet) Node() *Node {
	hst, err := dn.net.GenPeer()
	require.NoError(dn.T, err)
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
	nd := &Node{
		net:          dn,
		BlockService: blockservice.New(bstore, bs),
		Host:         hst,
	}
	dn.nodes = append(dn.nodes, nd)
	return nd
}

// ConnectAll connects all the peers on registered on the DagNet.
func (dn *DagNet) ConnectAll() {
	err := dn.net.LinkAll()
	require.NoError(dn.T, err)

	err = dn.net.ConnectAllButSelf()
	require.NoError(dn.T, err)
}

// Connect connects two given peer.
func (dn *DagNet) Connect(peerA, peerB peer.ID) {
	_, err := dn.net.LinkPeers(peerA, peerB)
	require.NoError(dn.T, err)
	_, err = dn.net.ConnectPeers(peerA, peerB)
	require.NoError(dn.T, err)
}

// Disconnect disconnects two peers.
// It does a hard disconnect, meaning that disconnected peers won't be able to reconnect on their own
// but only with DagNet.Connect or DagNet.ConnectAll.
func (dn *DagNet) Disconnect(peerA, peerB peer.ID) {
	err := dn.net.UnlinkPeers(peerA, peerB)
	require.NoError(dn.T, err)
	err = dn.net.DisconnectPeers(peerA, peerB)
	require.NoError(dn.T, err)
}

type SubNet struct {
	*DagNet
	nodes []*Node
}

func (dn *DagNet) SubNet() *SubNet {
	return &SubNet{dn, nil}
}

func (sn *SubNet) AddNode(nd *Node) {
	sn.nodes = append(sn.nodes, nd)
}

func (sn *SubNet) ConnectAll() {
	nodes := sn.nodes
	for _, n1 := range nodes {
		for _, n2 := range nodes {
			if n1 == n2 {
				continue
			}
			_, err := sn.net.LinkPeers(n1.ID(), n2.ID())
			require.NoError(sn.T, err)

			_, err = sn.net.ConnectPeers(n1.ID(), n2.ID())
			require.NoError(sn.T, err)
		}
	}
}

type TestBrokenAvailability struct {
	Root *share.Root
}

// NewTestBrokenAvailability returns an instance of Availability that
// allows for testing error cases during sampling.
//
// If the share.Root field is empty, it will return share.ErrNotAvailable on every call
// to SharesAvailable. Otherwise, it will only return ErrNotAvailable if the
// given Root hash matches the stored Root hash.
func NewTestBrokenAvailability() share.Availability {
	return &TestBrokenAvailability{}
}

func (b *TestBrokenAvailability) SharesAvailable(_ context.Context, root *share.Root) error {
	if b.Root == nil || bytes.Equal(b.Root.Hash(), root.Hash()) {
		return share.ErrNotAvailable
	}
	return nil
}

func (b *TestBrokenAvailability) ProbabilityOfAvailability() float64 {
	return 0
}

type TestSuccessfulAvailability struct {
}

// NewTestSuccessfulAvailability returns an Availability that always
// returns successfully when SharesAvailable is called.
func NewTestSuccessfulAvailability() share.Availability {
	return &TestSuccessfulAvailability{}
}

func (tsa *TestSuccessfulAvailability) SharesAvailable(context.Context, *share.Root) error {
	return nil
}

func (tsa *TestSuccessfulAvailability) ProbabilityOfAvailability() float64 {
	return 0
}
