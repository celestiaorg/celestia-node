package proofs

import (
	"context"
	"math"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/nmt"

	"github.com/celestiaorg/celestia-node/share/ipld"
)

func GetProofs(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root []byte,
	shareIdx,
	total int,
) (nmt.Proof, error) {
	rootCid := ipld.MustCidFromNamespacedSha256(root)
	proofPath := make([]cid.Cid, 0, int(math.Sqrt(float64(total))))
	proofPath, err := ipld.GetProof(ctx, bGetter, rootCid, proofPath, shareIdx, total)
	if err != nil {
		return nmt.Proof{}, err
	}

	rangeProofs := make([][]byte, 0, len(proofPath))
	for i := len(proofPath) - 1; i >= 0; i-- {
		node := ipld.NamespacedSha256FromCID(proofPath[i])
		rangeProofs = append(rangeProofs, node)
	}

	return nmt.NewInclusionProof(shareIdx, shareIdx+1, rangeProofs, true), nil
}
