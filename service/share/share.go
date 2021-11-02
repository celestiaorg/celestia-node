package share

import (
	"context"

	"github.com/celestiaorg/nmt/namespace"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"

	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/celestia-node/service/header"
)

// TODO(@Wondertan): We prefix real data of shares with namespaces to be able to recover them during erasure coding
//  recovery. However, that is storage and bandwidth overhead(8 bytes per each share) which we can avoid by getting
//  namespaces from CIDs stored in IPLD NMT Nodes, instead of encoding namespaces in erasure coding.
//
// TODO(@Wondertan): Ideally, Shares structure should be defined in a separate repository with
//  rsmt2d/nmt/celestia-core/celestia-node importing it. rsmt2d and nmt are already dependent on the notion of "share",
//  so why shouldn't we have a separated and global type for it to avoid the type mess with defining own share type in
//  each package.
//
// Share is
type Share = namespace.PrefixedData8

// Service provides as simple interface to access any DataSquare/Block Share on the network.
//
// All Get methods follow the following flow:
// 	* Check local storage for the requested Share.
// 		* If exist
// 			* Load from disk
//			* Return
//  	* If not
//  		* Find provider on the network
//      	* Fetch the Share from the provider
//			* Store the Share
//			* Return
type Service interface {
	// GetShare loads a Share committed to the given DataAvailabilityHeader by its Row anc Col coordinates in the
	// erasures data square or block.
	GetShare(ctx context.Context, dah header.DataAvailabilityHeader, row, col int) (Share, error)

	// GetShares loads all the Shares committed to the given DataAvailabilityHeader as a 2D array/slice.
	// It also optimistically executes erasure coding recovery.
	GetShares(context.Context, header.DataAvailabilityHeader) ([][]Share, error)

	// GetSharesByNamespace loads all the Shares committed to the given DataAvailabilityHeader as a 1D array/slice.
	GetSharesByNamespace(context.Context, header.DataAvailabilityHeader, namespace.ID) ([]Share, error)

	Start(context.Context) error
	Stop(context.Context) error
}

// NewService creates new basic share.Service.
func NewService(dag format.DAGService) Service {
	return &service{
		dag: dag,
	}
}

type service struct {
	dag format.DAGService

	// session is dag sub-session that applies optimization for fetching/loading related nodes, like shares
	// prefer session over dag for fetching nodes.
	session format.NodeGetter
	// cancel controls lifecycle of the session
	cancel context.CancelFunc
}

func (s *service) Start(context.Context) error {
	if s.session != nil || s.cancel != nil {
		return nil
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
		return nil
	}

	s.cancel()
	s.cancel = nil
	s.session = nil
	return nil
}

func (s *service) GetShare(ctx context.Context, dah header.DataAvailabilityHeader, row, col int) (Share, error) {
	rootCid, err := plugin.CidFromNamespacedSha256(dah.RowsRoots[row])
	if err != nil {
		return nil, err
	}

	nd, err := ipld.GetLeaf(ctx, s.dag, rootCid, col, len(dah.ColumnRoots))
	if err != nil {
		return nil, err
	}

	// we exclude one byte, as it is not part of the share, but encoding of IPLD NMT Node type.
	// TODO(@Wondertan): There is way to understand node type indirectly,
	//  without overhead of appending it to share(storing/fetching).
	return nd.RawData()[1:], nil
}

func (s *service) GetShares(ctx context.Context, dah header.DataAvailabilityHeader) ([][]Share, error) {
	panic("implement me")
}

func (s *service) GetSharesByNamespace(ctx context.Context, dah header.DataAvailabilityHeader, nid namespace.ID) ([]Share, error) {
	panic("implement me")
}
