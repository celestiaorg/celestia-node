package ipld

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
)

// GetShare fetches and returns the data for leaf `leafIndex` of root `rootCid`.
func GetShare(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	rootCid cid.Cid,
	leafIndex int,
	totalLeafs int, // this corresponds to the extended square width
) (Share, error) {
	nd, err := GetLeaf(ctx, bGetter, rootCid, leafIndex, totalLeafs)
	if err != nil {
		return nil, err
	}

	return leafToShare(nd), nil
}

// GetLeaf fetches and returns the raw leaf.
// It walks down the IPLD NMT tree until it finds the requested one.
func GetLeaf(ctx context.Context, bGetter blockservice.BlockGetter, root cid.Cid, leaf, total int) (ipld.Node, error) {
	// request the node
	nd, err := plugin.GetNode(ctx, bGetter, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		return plugin.GetNode(ctx, bGetter, lnks[0].Cid)
	}

	// route walk to appropriate children
	total /= 2 // as we are using binary tree, every step decreases total leaves in a half
	if leaf < total {
		root = lnks[0].Cid // if target leave on the left, go with walk down the first children
	} else {
		root, leaf = lnks[1].Cid, leaf-total // otherwise go down the second
	}

	// recursively walk down through selected children
	return GetLeaf(ctx, bGetter, root, leaf, total)
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
			s, err := GetLeaf(ctx, bGetter, root, index, len(shares))
			if err != nil {
				return nil, err
			}
			proof, err = GetProof(ctx, bGetter, root, proof, index, len(shares))
			if err != nil {
				return nil, err
			}
			proofs[index] = NewShareWithProof(index, s.RawData()[1:], proof)
		}
	}

	return proofs, nil
}

// GetProof fetches and returns the leaf's Merkle Proof.
// It walks down the IPLD NMT tree until it reaches the leaf and returns collected proof
func GetProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	proof []cid.Cid,
	leaf, total int,
) ([]cid.Cid, error) {
	// request the node
	nd, err := plugin.GetNode(ctx, bGetter, root)
	if err != nil {
		return nil, err
	}
	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		p := make([]cid.Cid, len(proof))
		copy(p, proof)
		return p, nil
	}

	// route walk to appropriate children
	total /= 2 // as we are using binary tree, every step decreases total leaves in a half
	if leaf < total {
		root = lnks[0].Cid // if target leave on the left, go with walk down the first children
		proof = append(proof, lnks[1].Cid)
	} else {
		root, leaf = lnks[1].Cid, leaf-total // otherwise go down the second
		proof, err = GetProof(ctx, bGetter, root, proof, leaf, total)
		if err != nil {
			return nil, err
		}
		return append(proof, lnks[0].Cid), nil
	}

	// recursively walk down through selected children
	return GetProof(ctx, bGetter, root, proof, leaf, total)
}

// leafToShare converts an NMT leaf into a Share.
func leafToShare(nd ipld.Node) Share {
	// * First byte represents the type of the node, and is unrelated to the actual share data
	// * Additional namespace is prepended so that parity data can be identified with a parity namespace, which we cut off
	return nd.RawData()[1+NamespaceSize:] // TODO(@Wondertan): Rework NMT/IPLD plugin to avoid the type byte
}
