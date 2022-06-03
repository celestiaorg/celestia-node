package share

import (
	"context"
	"math"
	"testing"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-routing/offline"
	format "github.com/ipfs/go-ipld-format"
	mdutils "github.com/ipfs/go-merkledag/test"
	record "github.com/libp2p/go-libp2p-record"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/pkg/wrapper"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
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
// the Availability is wrapped with cacheAvailability.
func RandLightLocalServiceWithSquare(t *testing.T, n int) (*Service, *Root) {
	bServ := mdutils.Bserv()
	ds := dssync.MutexWrap(ds.NewMapDatastore())
	ca := NewCacheAvailability(NewLightAvailability(bServ), ds)
	return NewService(bServ, ca), RandFillDAG(t, n, bServ)
}

// RandFullLocalServiceWithSquare is the same as RandFullServiceWithSquare, except
// the Availability is wrapped with cacheAvailability.
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
	na := ipld.NewNmtNodeAdder(context.TODO(), bServ, format.MaxSizeBatchOption(len(shares)))

	squareSize := uint32(math.Sqrt(float64(len(shares))))
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(squareSize), nmt.NodeVisitor(na.Visit))
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, rsmt2d.NewRSGF8Codec(), tree.Constructor)
	require.NoError(t, err)

	err = na.Commit()
	require.NoError(t, err)

	dah, err := header.DataAvailabilityHeaderFromExtendedData(eds)
	require.NoError(t, err)
	return &dah
}

// RandShares provides 'n' randomized shares prefixed with random namespaces.
func RandShares(t *testing.T, n int) []Share {
	return ipld.RandShares(t, n)
}

type DAGNet struct {
	ctx context.Context
	t   *testing.T
	net mocknet.Mocknet
}

func NewDAGNet(ctx context.Context, t *testing.T) *DAGNet {
	return &DAGNet{
		ctx: ctx,
		t:   t,
		net: mocknet.New(ctx),
	}
}

func (dn *DAGNet) RandLightService(n int) (*Service, *Root) {
	bServ, root := dn.RandDAG(n)
	return NewService(bServ, NewLightAvailability(bServ)), root
}

func (dn *DAGNet) RandFullService(n int) (*Service, *Root) {
	bServ, root := dn.RandDAG(n)
	return NewService(bServ, NewFullAvailability(bServ)), root
}

func (dn *DAGNet) RandDAG(n int) (blockservice.BlockService, *Root) {
	bServ := dn.CleanDAG()
	return bServ, RandFillDAG(dn.t, n, bServ)
}

func (dn *DAGNet) CleanService() *Service {
	bServ := dn.CleanDAG()
	return NewService(bServ, NewLightAvailability(bServ))
}

func (dn *DAGNet) CleanDAG() blockservice.BlockService {
	nd, err := dn.net.GenPeer()
	require.NoError(dn.t, err)

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	routing := offline.NewOfflineRouter(dstore, record.NamespacedValidator{})
	bs := bitswap.New(dn.ctx, network.NewFromIpfsHost(nd, routing), bstore, bitswap.ProvideEnabled(false))
	return blockservice.New(bstore, bs)
}

func (dn *DAGNet) ConnectAll() {
	err := dn.net.LinkAll()
	require.NoError(dn.t, err)

	err = dn.net.ConnectAllButSelf()
	require.NoError(dn.t, err)
}

// brokenAvailability allows to test error cases during sampling
type brokenAvailability struct {
}

func NewBrokenAvailability() Availability {
	return &brokenAvailability{}
}

func (b *brokenAvailability) SharesAvailable(context.Context, *Root) error {
	return ErrNotAvailable
}
