package shrex

import (
	"context"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
)

type Getter interface {
	GetSharesWithProofsByNamespace(
		ctx context.Context,
		rootHash []byte,
		rowRoots [][]byte,
		maxShares int,
		nID namespace.ID,
		collectProofs bool,
	) ([]share.SharesWithProof, error)
}
