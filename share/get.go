package share

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt/namespace"
)

// GetShare fetches and returns the data for leaf `leafIndex` of root `rootCid`.
func GetShare(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	rootCid cid.Cid,
	leafIndex int,
	totalLeafs int, // this corresponds to the extended square width
) (Share, error) {
	nd, err := ipld.GetLeaf(ctx, bGetter, rootCid, leafIndex, totalLeafs)
	if err != nil {
		return nil, err
	}

	return leafToShare(nd), nil
}

// GetShares walks the tree of a given root and puts shares into the given 'put' func.
// Does not return any error, and returns/unblocks only on success
// (got all shares) or on context cancellation.
func GetShares(ctx context.Context, bGetter blockservice.BlockGetter, root cid.Cid, shares int, put func(int, Share)) {
	ctx, span := tracer.Start(ctx, "get-shares")
	defer span.End()

	putNode := func(i int, leaf format.Node) {
		put(i, leafToShare(leaf))
	}
	ipld.GetLeaves(ctx, bGetter, root, shares, putNode)
}

// GetSharesByNamespace walks the tree of a given root and returns its shares within the given
// namespace.ID. If a share could not be retrieved, err is not nil, and the returned array
// contains nil shares in place of the shares it was unable to retrieve.
func GetSharesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) ([]Share, error) {
	ctx, span := tracer.Start(ctx, "get-shares-by-namespace")
	defer span.End()

	nodes, err := ipld.GetLeavesByNamespace(ctx, bGetter, root, nID, maxShares, false)
	if err != nil && nodes == nil {
		return nil, err
	}

	shares := make([]Share, 0, maxShares)
	for _, leaf := range nodes.Leaves {
		if leaf != nil {
			shares = append(shares, leafToShare(leaf))
		}
	}

	return shares, err
}

// leafToShare converts an NMT leaf into a Share.
func leafToShare(nd format.Node) Share {
	// * Additional namespace is prepended so that parity data can be identified with a parity
	// namespace, which we cut off
	return nd.RawData()[NamespaceSize:]
}
