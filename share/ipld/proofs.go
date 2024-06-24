package ipld

import (
	"context"
	"math"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/nmt"
)

func GetProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root []byte,
	shareIdx,
	total int,
) (nmt.Proof, error) {
	rootCid := MustCidFromNamespacedSha256(root)
	proofPath := make([]cid.Cid, 0, int(math.Sqrt(float64(total))))
	proofPath, err := getProof(ctx, bGetter, rootCid, proofPath, shareIdx, total)
	if err != nil {
		return nmt.Proof{}, err
	}

	rangeProofs := make([][]byte, 0, len(proofPath))
	for i := len(proofPath) - 1; i >= 0; i-- {
		node := NamespacedSha256FromCID(proofPath[i])
		rangeProofs = append(rangeProofs, node)
	}

	return nmt.NewInclusionProof(shareIdx, shareIdx+1, rangeProofs, true), nil
}

// getProof fetches and returns the leaf's Merkle Proof.
// It walks down the IPLD NMT tree until it reaches the leaf and returns collected proof
func getProof(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	proof []cid.Cid,
	leaf, total int,
) ([]cid.Cid, error) {
	// request the node
	nd, err := GetNode(ctx, bGetter, root)
	if err != nil {
		return nil, err
	}
	// look for links
	lnks := nd.Links()
	if len(lnks) == 0 {
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
		proof, err = getProof(ctx, bGetter, root, proof, leaf, total)
		if err != nil {
			return nil, err
		}
		return append(proof, lnks[0].Cid), nil
	}

	// recursively walk down through selected children
	return getProof(ctx, bGetter, root, proof, leaf, total)
}
