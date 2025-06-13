package proof

import (
	"fmt"

	"github.com/celestiaorg/celestia-app/v4/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
)

func GenerateSharesProofs(
	row, fromCol, toCol, size int,
	rowShares []libshare.Share,
) (*nmt.Proof, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(size), uint(row))

	for i, shr := range rowShares {
		if err := tree.Push(shr.ToBytes()); err != nil {
			return nil, fmt.Errorf("failed to build tree at share index %d (row %d): %w", i, row, err)
		}
	}

	proof, err := tree.ProveRange(fromCol, toCol)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof for row %d, range %d-%d: %w", row, fromCol, toCol, err)
	}
	return &proof, nil
}
