package ipld

import (
	"context"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/ipld/plugin"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

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

// GetLeafData fetches and returns the data for leaf leafIndex of root rootCid.
// It stops and returns an error if the provided context is canceled before
// finishing
func GetLeafData(
	ctx context.Context,
	rootCid cid.Cid,
	leafIndex uint32,
	totalLeafs uint32, // this corresponds to the extended square width
	dag ipld.NodeGetter,
) ([]byte, error) {
	nd, err := GetLeaf(ctx, dag, rootCid, int(leafIndex), int(totalLeafs))
	if err != nil {
		return nil, err
	}

	return nd.RawData()[1:], nil
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

// GetLeavesByNamespace returns all the shares from the given DataAvailabilityHeader root
// with the given namespace.ID.
func GetLeavesByNamespace(
	ctx context.Context,
	dag ipld.NodeGetter,
	root cid.Cid,
	nID namespace.ID,
) (out []ipld.Node, err error) {
	rootH := plugin.NamespacedSha256FromCID(root)
	if nID.Less(nmt.MinNamespace(rootH, nID.Size())) || !nID.LessOrEqual(nmt.MaxNamespace(rootH, nID.Size())) {
		return nil, ErrNotFoundInRange
	}
	// request the node
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return
	}
	// check links
	lnks := nd.Links()
	if len(lnks) == 1 {
		// if there is one link, then this is a leaf node, so just return it
		out = append(out, nd)
		return
	}
	// if there are some links, then traverse them
	for _, lnk := range nd.Links() {
		nds, err := GetLeavesByNamespace(ctx, dag, lnk.Cid, nID)
		if err != nil {
			if err == ErrNotFoundInRange {
				// There is always right and left child and it is ok if one of them does not have a required nID.
				continue
			}
		}
		out = append(out, nds...)
	}
	return out, err
}

// getSubtreeLeavesWithProof is downloading leaf data with it's proof by calling getLeavesWithProofs
func getSubtreeLeavesWithProof(ctx context.Context, root cid.Cid, dag ipld.NodeGetter, isLeftSubtree bool) ([]*NamespacedShareWithProof, error) {
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, err
	}
	leaves := make([]ipld.Node, 0)
	proofs := make([][]cid.Cid, 0)
	path := make([]cid.Cid, 0)
	nameSpacedShares := make([]*NamespacedShareWithProof, 0)
	if isLeftSubtree {
		path = append(path, nd.Links()[1].Cid)
		proofs, leaves, err = getLeavesWithProofs(ctx, nd.Links()[0].Cid, dag, proofs, leaves, path)
		for index, leaf := range leaves {
			nameSpacedShares = append(nameSpacedShares, NewShareWithProof(index, leaf, proofs[index]))
		}
		return nameSpacedShares, nil
	}
	proofs, leaves, err = getLeavesWithProofs(ctx, nd.Links()[1].Cid, dag, proofs, leaves, path)
	for index, leaf := range leaves {
		proofs[index] = append(proofs[index], nd.Links()[0].Cid)
		nameSpacedShares = append(nameSpacedShares, NewShareWithProof(index, leaf, proofs[index]))
	}
	return nil, err
}

// getLeavesWithProofs is recursively going from the given root to the leaf, collecting the opposite subnode hashes as proofs.
func getLeavesWithProofs(ctx context.Context, root cid.Cid, dag ipld.NodeGetter, proofs [][]cid.Cid, leaves []ipld.Node, path []cid.Cid) ([][]cid.Cid, []ipld.Node, error) {
	nd, err := dag.Get(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	lnks := nd.Links()
	if len(lnks) == 1 {
		// in case there is only one we reached tree's bottom, so finally request the leaf.
		nd, err = dag.Get(ctx, lnks[0].Cid)
		if err != nil {
			return nil, nil, err
		}
		p := make([]cid.Cid, len(path))
		copy(p, path)
		return append(proofs, p), append(leaves, nd), nil
	}

	for index, node := range lnks {
		path := path
		if index == leftSubnode {
			// appending right side for the left subnode
			path = append(path, lnks[rightSubNode].Cid)
		}
		leavesAmount := len(leaves)
		//means we are going left
		proofs, leaves, err = getLeavesWithProofs(ctx, node.Cid, dag, proofs, leaves, path)
		if err != nil {
			return nil, nil, err
		}
		if index == rightSubNode {
			// appending left subnode that were fetched in previous getLeavesWithProofs
			for idx := leavesAmount; idx < len(leaves); idx++ {
				proofs[idx] = append(proofs[idx], lnks[leftSubnode].Cid)
			}
		}

	}

	return proofs, leaves, err
}
