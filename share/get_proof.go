package share

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// ShareWithProof contains data with corresponding Merkle Proof
type SharesWithProofs struct {
	// Share is a full data including namespace
	Shares []Share
	// Proof is a Merkle Proof of current share
	Proof nmt.Proof
}

// GetSharesWithProofsByNamespace walks the tree of a given root and returns its shares within the given namespace.ID.
// If a share could not be retrieved, err is not nil, and the returned array
// contains nil shares in place of the shares it was unable to retrieve.
func GetSharesWithProofsByNamespace(
	ctx context.Context,
	bGetter blockservice.BlockGetter,
	root cid.Cid,
	nID namespace.ID,
	maxShares int,
) (*SharesWithProofs, error) {
	ctx, span := tracer.Start(ctx, "get-shares-by-namespace")
	defer span.End()

	nodes, err := ipld.GetLeavesWithProofsByNamespace(ctx, bGetter, root, nID, maxShares)
	if nodes == nil {
		return nil, err
	}

	shares := make([]Share, 0, nodes.ProofEnd-nodes.ProofStart)
	for _, leaf := range nodes.Leaves {
		if leaf != nil {
			shares = append(shares, leafToShare(leaf))
		}
	}

	// pack proofs for nmt.Proof
	rangeProofs := make([][]byte, 0, len(nodes.Proofs))
	for i := len(nodes.Proofs) - 1; i >= 0; i-- {
		if nodes.Proofs[i] != nil {
			rangeProofs = append(rangeProofs, nodes.Proofs[i])
		}

	}

	return &SharesWithProofs{
		Shares: shares,
		Proof:  nmt.NewInclusionProof(nodes.ProofStart, nodes.ProofEnd, rangeProofs, true),
	}, nil
}
