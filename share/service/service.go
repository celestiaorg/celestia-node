package service

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/celestia-node/share/retriever"

	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// Share is a fixed-size data chunk associated with a namespace ID, whose data will be erasure-coded and committed
// to in Namespace Merkle trees.
type Share = share.Share

// TODO(@Wondertan): Simple thread safety for Start and Stop would not hurt.
type ShareService struct {
	share.Availability
	rtrv  *retriever.Retriever
	bServ blockservice.BlockService
	// session is blockservice sub-session that applies optimization for fetching/loading related nodes, like shares
	// prefer session over blockservice for fetching nodes.
	session blockservice.BlockGetter
	cancel  context.CancelFunc
}

// NewService creates a new basic share.Module.
func NewShareService(bServ blockservice.BlockService, avail share.Availability) *ShareService {
	return &ShareService{
		rtrv:         retriever.NewRetriever(bServ),
		Availability: avail,
		bServ:        bServ,
	}
}

func (s *ShareService) Start(context.Context) error {
	if s.session != nil || s.cancel != nil {
		return fmt.Errorf("share: service already started")
	}

	// NOTE: The ctx given as param is used to control Start flow and only needed when Start is blocking,
	// but this one is not.
	//
	// The newer context here is created to control lifecycle of the session and peer discovery.
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.session = blockservice.NewSession(ctx, s.bServ)
	return nil
}

func (s *ShareService) Stop(context.Context) error {
	if s.session == nil || s.cancel == nil {
		return fmt.Errorf("share: service already stopped")
	}

	s.cancel()
	s.cancel = nil
	s.session = nil
	return nil
}

func (s *ShareService) GetShare(ctx context.Context, dah *share.Root, row, col int) (Share, error) {
	root, leaf := share.Translate(dah, row, col)
	nd, err := share.GetShare(ctx, s.bServ, root, leaf, len(dah.RowsRoots))
	if err != nil {
		return nil, err
	}

	return nd, nil
}

func (s *ShareService) GetShares(ctx context.Context, root *share.Root) ([][]Share, error) {
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

// GetSharesByNamespace iterates over a square's row roots and accumulates the found shares in the given namespace.ID.
func (s *ShareService) GetSharesByNamespace(ctx context.Context, root *share.Root, nID namespace.ID) ([]Share, error) {
	err := share.SanityCheckNID(nID)
	if err != nil {
		return nil, err
	}
	rowRootCIDs := make([]cid.Cid, 0)
	for _, row := range root.RowsRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			rowRootCIDs = append(rowRootCIDs, ipld.MustCidFromNamespacedSha256(row))
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
			shares[i], err = share.GetSharesByNamespace(ctx, s.bServ, rootCID, nID, len(root.RowsRoots))
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
