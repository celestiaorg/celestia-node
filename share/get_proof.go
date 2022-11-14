package share

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

// SharesWithProofs contains data with corresponding Merkle Proof
type SharesWithProofs struct {
	// Share is a full data including namespace
	Shares []Share
	// Proof is a Merkle Proof of current share
	Proof nmt.Proof
}

// GetSharesWithProofsByNamespace walks the tree of a given root and returns its shares within the
// given namespace.ID. If a share could not be retrieved, err is not nil, and the returned array
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

	nodes, err := ipld.GetLeavesByNamespace(ctx, bGetter, root, nID, maxShares, true)
	if nodes == nil {
		return nil, err
	}

	shares := make([]Share, 0, nodes.ProofEnd-nodes.ProofStart)
	for _, leaf := range nodes.Leaves {
		if leaf != nil {
			shares = append(shares, leafToShare(leaf))
		}
	}

	return &SharesWithProofs{
		Shares: shares,
		Proof:  nmt.NewInclusionProof(nodes.ProofStart, nodes.ProofEnd, nodes.Proofs, true),
	}, nil
}
