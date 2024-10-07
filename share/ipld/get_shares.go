package ipld

import (
	"context"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"

	gosquare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
)

// GetShare fetches and returns the data for leaf `leafIndex` of root `rootCid`.
func GetShare(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	rootCid cid.Cid,
	leafIndex int,
	totalLeafs int, // this corresponds to the extended square width
) (gosquare.Share, error) {
	nd, err := GetLeaf(ctx, bGetter, rootCid, leafIndex, totalLeafs)
	if err != nil {
		return gosquare.Share{}, err
	}

	sh, err := gosquare.NewShare(nd.RawData()[gosquare.NamespaceSize:])
	if err != nil {
		return gosquare.Share{}, err
	}
	return *sh, nil
}

// GetShares walks the tree of a given root and puts shares into the given 'put' func.
// Does not return any error, and returns/unblocks only on success
// (got all shares) or on context cancellation.
func GetShares(ctx context.Context, bg blockservice.BlockGetter, root cid.Cid, shares int, put func(int, []byte)) {
	putNode := func(i int, leaf format.Node) {
		put(i, leaf.RawData()[gosquare.NamespaceSize:])
	}
	GetLeaves(ctx, bg, root, shares, putNode)
}

// GetSharesByNamespace walks the tree of a given root and returns its shares within the given
// Namespace. If a share could not be retrieved, err is not nil, and the returned array
// contains nil shares in place of the shares it was unable to retrieve.
func GetSharesByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root []byte,
	namespace gosquare.Namespace,
	maxShares int,
) ([]gosquare.Share, *nmt.Proof, error) {
	rootCid := MustCidFromNamespacedSha256(root)
	data := NewNamespaceData(maxShares, namespace, WithLeaves(), WithProofs())
	err := data.CollectLeavesByNamespace(ctx, bGetter, rootCid)
	if err != nil {
		return nil, nil, err
	}

	leaves := data.Leaves()
	if len(leaves) == 0 {
		return nil, data.Proof(), nil
	}

	shares := make([]gosquare.Share, len(leaves))
	for i, leaf := range leaves {
		if leaf != nil {
			sh, err := gosquare.NewShare(leaf.RawData()[gosquare.NamespaceSize:])
			if err != nil {
				return nil, nil, err
			}
			shares[i] = *sh
		}
	}
	return shares, data.Proof(), nil
}
