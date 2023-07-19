package availability_test

import (
	"context"
	"testing"

	"github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-routing/offline"
	"github.com/ipfs/go-libipfs/bitswap"
	"github.com/ipfs/go-libipfs/bitswap/network"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

// RandFillBS fills the given BlockService with a random block of a given size.
func RandFillBS(t *testing.T, n int, bServ blockservice.BlockService) *share.Root {
	shares := sharetest.RandShares(t, n*n)
	return FillBS(t, bServ, shares)
}

// FillBS fills the given BlockService with the given shares.
func FillBS(t *testing.T, bServ blockservice.BlockService, shares []share.Share) *share.Root {
	eds, err := ipld.AddShares(context.TODO(), shares, bServ)
	require.NoError(t, err)
	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)
	return &dah
}

type TestNode struct {
	net *TestDagNet
	share.Getter
	share.Availability
	blockservice.BlockService
	host.Host
}

// ClearStorage cleans up the storage of the node.
func (n *TestNode) ClearStorage() {
	keys, err := n.Blockstore().AllKeysChan(n.net.ctx)
	require.NoError(n.net.T, err)

	for k := range keys {
		err := n.DeleteBlock(n.net.ctx, k)
		require.NoError(n.net.T, err)
	}
}

type TestDagNet struct {
	ctx   context.Context
	T     *testing.T
	net   mocknet.Mocknet
	nodes []*TestNode
}

// NewTestDAGNet creates a new testing swarm utility to spawn different nodes and test how they
// interact and/or exchange data.
func NewTestDAGNet(ctx context.Context, t *testing.T) *TestDagNet {
	return &TestDagNet{
		ctx: ctx,
		T:   t,
		net: mocknet.New(),
	}
}

// NewTestNodeWithBlockstore creates a new plain TestNode with the given blockstore that can serve
// and request data.
func (dn *TestDagNet) NewTestNodeWithBlockstore(dstore ds.Datastore, bstore blockstore.Blockstore) *TestNode {
	hst, err := dn.net.GenPeer()
	require.NoError(dn.T, err)
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
	nd := &TestNode{
		net:          dn,
		BlockService: blockservice.New(bstore, bs),
		Host:         hst,
	}
	dn.nodes = append(dn.nodes, nd)
	return nd
}

// NewTestNode creates a plain network node that can serve and request data.
func (dn *TestDagNet) NewTestNode() *TestNode {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	return dn.NewTestNodeWithBlockstore(dstore, bstore)
}

// ConnectAll connects all the peers on registered on the TestDagNet.
func (dn *TestDagNet) ConnectAll() {
	err := dn.net.LinkAll()
	require.NoError(dn.T, err)

	err = dn.net.ConnectAllButSelf()
	require.NoError(dn.T, err)
}

// Connect connects two given peers.
func (dn *TestDagNet) Connect(peerA, peerB peer.ID) {
	_, err := dn.net.LinkPeers(peerA, peerB)
	require.NoError(dn.T, err)
	_, err = dn.net.ConnectPeers(peerA, peerB)
	require.NoError(dn.T, err)
}

// Disconnect disconnects two peers.
// It does a hard disconnect, meaning that disconnected peers won't be able to reconnect on their
// own but only with DagNet.Connect or TestDagNet.ConnectAll.
func (dn *TestDagNet) Disconnect(peerA, peerB peer.ID) {
	err := dn.net.UnlinkPeers(peerA, peerB)
	require.NoError(dn.T, err)
	err = dn.net.DisconnectPeers(peerA, peerB)
	require.NoError(dn.T, err)
}

type SubNet struct {
	*TestDagNet
	nodes []*TestNode
}

func (dn *TestDagNet) SubNet() *SubNet {
	return &SubNet{dn, nil}
}

func (sn *SubNet) AddNode(nd *TestNode) {
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
