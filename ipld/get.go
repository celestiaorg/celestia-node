package ipld

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/nmt/namespace"
)

// ErrByzantine is an error converted from rsmt2d.ByzantineRow/Col +
// Merkle Proof of each share.
type ErrByzantine struct {
	Index  uint8
	Shares []*ShareWithProof
	// TODO(@vgokivs): Change to enum type and rename to Axis after
	// updating rsmt2d
	IsRow bool
}

func (e *ErrByzantine) Error() string {
	return fmt.Sprintf("byzantine error. isRow:%v, Index:%v", e.IsRow, e.Index)
}

// GetLeaves gets all the leaves under the given root. It recursively starts traversing the DAG to find all the leafs.
// It also takes a pre-allocated slice 'leaves'.
func GetLeaves(ctx context.Context, dag ipld.NodeGetter, root cid.Cid, leaves []ipld.Node) ([]ipld.Node, error) {
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		nd, err = dag.Get(ctx, lnks[0].Cid)
		if err != nil {
			return nil, err
		}

		return append(leaves, nd), nil
	}

	for _, node := range lnks {
		// recursively walk down through selected children to request the leaves
		leaves, err = GetLeaves(ctx, dag, node.Cid, leaves)
		if err != nil {
			return nil, err
		}
	}
	return leaves, nil
}

// GetShare fetches and returns the data for leaf `leafIndex` of root `rootCid`.
func GetShare(
	ctx context.Context,
	dag ipld.NodeGetter,
	rootCid cid.Cid,
	leafIndex int,
	totalLeafs int, // this corresponds to the extended square width
) (Share, error) {
	nd, err := GetLeaf(ctx, dag, rootCid, leafIndex, totalLeafs)
	if err != nil {
		return nil, err
	}

	return leafToShare(nd), nil
}

// GetLeaf fetches and returns the raw leaf.
// It walks down the IPLD NMT tree until it finds the requested one.
func GetLeaf(ctx context.Context, dag ipld.NodeGetter, root cid.Cid, leaf, total int) (ipld.Node, error) {
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}

	// look for links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		return dag.Get(ctx, lnks[0].Cid)
	}

	// route walk to appropriate children
	total /= 2 // as we are using binary tree, every step decreases total leaves in a half
	if leaf < total {
		root = lnks[0].Cid // if target leave on the left, go with walk down the first children
	} else {
		root, leaf = lnks[1].Cid, leaf-total // otherwise go down the second
	}

	// recursively walk down through selected children
	return GetLeaf(ctx, dag, root, leaf, total)
}

// GetProofsForShares fetches Merkle proofs for the given shares
// and returns the result as an array of ShareWithProof.
func GetProofsForShares(
	ctx context.Context,
	dag ipld.NodeGetter,
	root cid.Cid,
	shares [][]byte,
) ([]*ShareWithProof, error) {
	proofs := make([]*ShareWithProof, len(shares))
	for index, share := range shares {
		if share != nil {
			proof := make([]cid.Cid, 0)
			// TODO(@vgonkivs): Combine GetLeafData and GetProof in one function as the are traversing the same tree.
			// Add options that will control what data will be fetched.
			s, err := GetLeaf(ctx, dag, root, index, len(shares))
			if err != nil {
				return nil, err
			}
			proof, err = GetProof(ctx, dag, root, proof, index, len(shares))
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
	dag ipld.NodeGetter,
	root cid.Cid,
	proof []cid.Cid,
	leaf, total int,
) ([]cid.Cid, error) {
	// request the node
	nd, err := dag.Get(ctx, root)
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
		proof, err = GetProof(ctx, dag, root, proof, leaf, total)
		if err != nil {
			return nil, err
		}
		return append(proof, lnks[0].Cid), nil
	}

	// recursively walk down through selected children
	return GetProof(ctx, dag, root, proof, leaf, total)
}

// GetSharesByNamespace returns all the shares from the given root
// with the given namespace.ID.
func GetSharesByNamespace(
	ctx context.Context,
	dag ipld.NodeGetter,
	root cid.Cid,
	nID namespace.ID,
) ([]Share, error) {
	leaves, err := GetLeavesByNamespace(ctx, dag, root, nID)
	if err != nil {
		return nil, err
	}

	shares := make([]Share, len(leaves))
	for i, leaf := range leaves {
		shares[i] = leafToShare(leaf)
	}

	return shares, nil
}

// GetLeavesByNamespace returns all the leaves from the given root with the given namespace.ID.
// If nothing is found it returns both data and err as nil.
func GetLeavesByNamespace(
	ctx context.Context,
	dag ipld.NodeGetter,
	root cid.Cid,
	nID namespace.ID,
) ([]ipld.Node, error) {
	rootH := plugin.NamespacedSha256FromCID(root)
	if nID.Less(nmt.MinNamespace(rootH, nID.Size())) || !nID.LessOrEqual(nmt.MaxNamespace(rootH, nID.Size())) {
		return nil, nil
	}
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}
	// check links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// if there is one link, then this is a leaf node, so just return it
		return []ipld.Node{nd}, nil
	}
	// if there are some links, then traverse them
	var out []ipld.Node
	for _, lnk := range nd.Links() {
		nds, err := GetLeavesByNamespace(ctx, dag, lnk.Cid, nID)
		if err != nil {
			return out, err
		}
		out = append(out, nds...)
	}
	return out, nil
}

// leafToShare converts an NMT leaf into a Share.
func leafToShare(nd ipld.Node) Share {
	// * First byte represents the type of the node, and is unrelated to the actual share data
	// * Additional namespace is prepended so that parity data can be identified with a parity namespace, which we cut off
	return nd.RawData()[1+NamespaceSize:] // TODO(@Wondertan): Rework NMT/IPLD plugin to avoid the type byte
}
