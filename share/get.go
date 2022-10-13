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

// GetSharesByNamespace walks the tree of a given root and returns its shares within the given namespace.ID.
// If a share could not be retrieved, err is not nil, and the returned array
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

	leaves, err := ipld.GetLeavesByNamespace(ctx, bGetter, root, nID, maxShares)
	if err != nil && leaves == nil {
		return nil, err
	}

	shares := make([]Share, len(leaves))
	for i, leaf := range leaves {
		if leaf != nil {
			shares[i] = leafToShare(leaf)
		}
	}

	return shares, err
}

// GetProofsForShares fetches Merkle proofs for the given shares
// and returns the result as an array of ShareWithProof.
func GetProofsForShares(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	shares [][]byte,
) ([]*ShareWithProof, error) {
	proofs := make([]*ShareWithProof, len(shares))
	for index, share := range shares {
		if share != nil {
			proof := make([]cid.Cid, 0)
			// TODO(@vgonkivs): Combine GetLeafData and GetProof in one function as the are traversing the same tree.
			// Add options that will control what data will be fetched.
			s, err := ipld.GetLeaf(ctx, bGetter, root, index, len(shares))
			if err != nil {
				return nil, err
			}
			proof, err = ipld.GetProof(ctx, bGetter, root, proof, index, len(shares))
			if err != nil {
				return nil, err
			}
			proofs[index] = NewShareWithProof(index, s.RawData()[1:], proof)
		}
	}

	return proofs, nil
}

// leafToShare converts an NMT leaf into a Share.
func leafToShare(nd format.Node) Share {
	// * First byte represents the type of the node, and is unrelated to the actual share data
	// * Additional namespace is prepended so that parity data can be identified with a parity namespace, which we cut off
	return nd.RawData()[1+NamespaceSize:] // TODO(@Wondertan): Rework NMT/IPLD plugin to avoid the type byte
}
