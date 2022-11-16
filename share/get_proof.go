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
	// Shares are the full data including namespace
	Shares []Share
	// Proof is a Merkle Proof for the corresponding Shares
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
	ctx, span := tracer.Start(ctx, "get-shares-by-namespace-with-proof")
	defer span.End()

	proof := new(ipld.Proof)
	leaves, err := ipld.GetLeavesByNamespace(ctx, bGetter, root, nID, maxShares, proof)
	if err != nil {
		return nil, err
	}

	shares := make([]Share, 0, proof.End-proof.Start)
	for _, leaf := range leaves {
		if leaf != nil {
			shares = append(shares, leafToShare(leaf))
		}
	}

	return &SharesWithProofs{
		Shares: shares,
		Proof:  nmt.NewInclusionProof(proof.Start, proof.End, proof.Nodes, ipld.NMTIgnoreMaxNamespace),
	}, nil
}
