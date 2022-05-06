package share

import (
	"context"
	"fmt"
	"math/rand"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-merkledag"
	"github.com/tendermint/tendermint/pkg/da"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/celestia-node/node/rpc"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

var log = logging.Logger("share")

// Share is a fixed-size data chunk associated with a namespace ID, whose data will be erasure-coded and committed
// to in Namespace Merkle trees.
type Share = ipld.Share

// GetID extracts namespace ID out of a Share.
var GetID = ipld.ShareID

// GetData extracts data out of a Share.
var GetData = ipld.ShareData

// Root represents root commitment to multiple Shares.
// In practice, it is a commitment to all the Data in a square.
type Root = da.DataAvailabilityHeader

// Service provides a simple interface to access any data square or block share on the network.
//
// All Get methods follow the following flow:
// 	* Check local storage for the requested Share.
// 		* If exists
// 			* Load from disk
//			* Return
//  	* If not
//  		* Find provider on the network
//      	* Fetch the Share from the provider
//			* Store the Share
//			* Return
type Service interface {
	Availability

	// GetShare loads a Share committed to the given DataAvailabilityHeader by its Row and Column coordinates in the
	// erasure coded data square or block.
	GetShare(ctx context.Context, root *Root, row, col int) (Share, error)

	// GetShares loads all the Shares committed to the given DataAvailabilityHeader as a 2D array/slice.
	// It also optimistically executes erasure coding recovery.
	GetShares(context.Context, *Root) ([][]Share, error)

	// GetSharesByNamespace loads all the Shares of the given namespace.ID committed to the given
	// DataAvailabilityHeader as a 1D array/slice.
	GetSharesByNamespace(context.Context, *Root, namespace.ID) ([]Share, error)

	// Start starts the Service.
	Start(context.Context) error

	// Stop stops the Service.
	Stop(context.Context) error

	// RegisterEndpoints registers the service's available endpoints on the RPC
	// server.
	RegisterEndpoints(rpc *rpc.Server)
}

// NewService creates new basic share.Service.
func NewService(dag format.DAGService, avail Availability) Service {
	return &service{
		rtrv:         ipld.NewRetriever(dag, ipld.DefaultRSMT2DCodec()),
		Availability: avail,
		dag:          dag,
	}
}

// TODO(@Wondertan): Simple thread safety for Start and Stop would not hurt.
type service struct {
	Availability
	rtrv *ipld.Retriever
	dag  format.DAGService
	// session is dag sub-session that applies optimization for fetching/loading related nodes, like shares
	// prefer session over dag for fetching nodes.
	session format.NodeGetter
	// cancel controls lifecycle of the session
	cancel context.CancelFunc
}

func (s *service) Start(context.Context) error {
	if s.session != nil || s.cancel != nil {
		return fmt.Errorf("share: Service already started")
	}

	// NOTE: The ctx given as param is used to control Start flow and only needed when Start is blocking,
	// but this one is not.
	//
	// The newer context here is created to control lifecycle of the session.
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.session = merkledag.NewSession(ctx, s.dag)
	return nil
}

func (s *service) Stop(context.Context) error {
	if s.session == nil || s.cancel == nil {
		return fmt.Errorf("share: Service already stopped")
	}

	s.cancel()
	s.cancel = nil
	s.session = nil
	return nil
}

func (s *service) GetShare(ctx context.Context, dah *Root, row, col int) (Share, error) {
	root, leaf := translate(dah, row, col)
	nd, err := ipld.GetShare(ctx, s.dag, root, leaf, len(dah.RowsRoots))
	if err != nil {
		return nil, err
	}

	return nd, nil
}

func (s *service) GetShares(ctx context.Context, root *Root) ([][]Share, error) {
	eds, err := s.rtrv.Retrieve(ctx, root)
	if err != nil {
		return nil, err
	}

	origWidth := int(eds.Width() / 2)
	shares := make([][]Share, origWidth)

	for i := 0; i < origWidth; i++ {
		row := eds.Row(uint(i))
		shares[i] = make([]Share, origWidth)
		for j := 0; j < origWidth; j++ {
			shares[i][j] = row[j]
		}
	}

	return shares, nil
}

func (s *service) GetSharesByNamespace(ctx context.Context, root *Root, nID namespace.ID) ([]Share, error) {
	rowRootCIDs := make([]cid.Cid, 0)
	for _, row := range root.RowsRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			rowRootCIDs = append(rowRootCIDs, plugin.MustCidFromNamespacedSha256(row))
		}
	}
	if len(rowRootCIDs) == 0 {
		return nil, nil
	}

	errGroup, ctx := errgroup.WithContext(ctx)
	shares := make([][]Share, len(rowRootCIDs))
	for i, rootCID := range rowRootCIDs {
		// shadow loop variables, to ensure correct values are captured
		i, rootCID := i, rootCID
		errGroup.Go(func() (err error) {
			shares[i], err = ipld.GetSharesByNamespace(ctx, s.dag, rootCID, nID)
			return
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	// we don't know the amount of shares in the namespace, so we cannot preallocate properly
	// TODO(@Wondertan): Consider improving encoding schema for data in the shares that will also include metadata
	// 	with the amount of shares. If we are talking about plenty of data here, proper preallocation would make a
	// 	difference
	var out []Share
	for i := 0; i < len(rowRootCIDs); i++ {
		out = append(out, shares[i]...)
	}

	return out, nil
}

// translate transforms square coordinates into IPLD NMT tree path to a leaf node.
// It also adds randomization to evenly spread fetching from Rows and Columns.
func translate(dah *Root, row, col int) (cid.Cid, int) {
	if rand.Intn(2) == 0 { //nolint:gosec
		return plugin.MustCidFromNamespacedSha256(dah.ColumnRoots[col]), row
	}

	return plugin.MustCidFromNamespacedSha256(dah.RowsRoots[row]), col
}
