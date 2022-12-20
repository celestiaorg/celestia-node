package getters

import (
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/nmt/namespace"
)

var _ share.Getter = (*IPLDGetter)(nil)

type IPLDGetter struct {
	rtrv  *eds.Retriever
	bServ blockservice.BlockService
}

func NewIPLDGetter(bServ blockservice.BlockService) *IPLDGetter {
	return &IPLDGetter{
		rtrv:  eds.NewRetriever(bServ),
		bServ: bServ,
	}
}

func (ig *IPLDGetter) GetShare(ctx context.Context, dah *share.Root, row, col int) (share.Share, error) {
	root, leaf := ipld.Translate(dah, row, col)
	nd, err := share.GetShare(ctx, ig.bServ, root, leaf, len(dah.RowsRoots))
	if err != nil {
		return nil, err
	}

	return nd, nil
}

func (ig *IPLDGetter) GetShares(ctx context.Context, root *share.Root) ([][]share.Share, error) {
	eds, err := ig.rtrv.Retrieve(ctx, root)
	if err != nil {
		return nil, err
	}

	origWidth := int(eds.Width() / 2)
	shares := make([][]share.Share, origWidth)

	for i := 0; i < origWidth; i++ {
		row := eds.Row(uint(i))
		shares[i] = make([]share.Share, origWidth)
		for j := 0; j < origWidth; j++ {
			shares[i][j] = row[j]
		}
	}

	return shares, nil
}

func (ig *IPLDGetter) GetSharesByNamespace(
	ctx context.Context,
	root *share.Root,
	nID namespace.ID,
) ([]share.Share, error) {
	if len(nID) != share.NamespaceSize {
		return nil, fmt.Errorf("expected namespace ID of size %d, got %d", share.NamespaceSize, len(nID))
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
	shares := make([][]share.Share, len(rowRootCIDs))
	for i, rootCID := range rowRootCIDs {
		// shadow loop variables, to ensure correct values are captured
		i, rootCID := i, rootCID
		errGroup.Go(func() (err error) {
			shares[i], err = share.GetSharesByNamespace(ctx, ig.bServ, rootCID, nID, len(root.RowsRoots), nil)
			return
		})
	}

	if err := errGroup.Wait(); err != nil {
		return nil, err
	}

	// we don't know the amount of shares in the namespace, so we cannot preallocate properly
	// TODO(@Wondertan): Consider improving encoding schema for data in the shares that will also
	// include metadata 	with the amount of shares. If we are talking about plenty of data here, proper
	// preallocation would make a 	difference
	var out []share.Share
	for i := 0; i < len(rowRootCIDs); i++ {
		out = append(out, shares[i]...)
	}

	return out, nil
}
