package share

import (
	"crypto/sha256"
	"fmt"
	"github.com/celestiaorg/nmt"
)

// NamespacedShares represents all shares with proofs within a specific namespace of an EDS.
type NamespacedShares []NamespacedRow

// Flatten returns the concatenated slice of all NamespacedRow shares.
func (ns NamespacedShares) Flatten() []Share {
	shares := make([]Share, 0)
	for _, row := range ns {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// NamespacedRow represents all shares with proofs within a specific namespace of a single EDS row.
type NamespacedRow struct {
	Shares []Share    `json:"shares"`
	Proof  *nmt.Proof `json:"proof"`
}

// Verify validates NamespacedShares by checking every row with nmt inclusion proof.
func (ns NamespacedShares) Verify(root *Root, namespace Namespace) error {
	var originalRoots [][]byte
	for _, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			originalRoots = append(originalRoots, row)
		}
	}

	if len(originalRoots) != len(ns) {
		return fmt.Errorf("amount of rows differs between root and namespace shares: expected %d, got %d",
			len(originalRoots), len(ns))
	}

	for i, row := range ns {
		if row.Proof == nil && row.Shares == nil {
			return fmt.Errorf("row verification failed: no proofs and shares")
		}
		// verify row data against row hash from original root
		if !row.Verify(originalRoots[i], namespace) {
			return fmt.Errorf("row verification failed: row %d doesn't match original root: %s", i, root.String())
		}
	}
	return nil
}

// Verify validates the row using nmt inclusion proof.
func (row *NamespacedRow) Verify(rowRoot []byte, namespace Namespace) bool {
	// construct nmt leaves from shares by prepending namespace
	leaves := make([][]byte, 0, len(row.Shares))
	for _, shr := range row.Shares {
		leaves = append(leaves, append(GetNamespace(shr), shr...))
	}

	// verify namespace
	return row.Proof.VerifyNamespace(
		sha256.New(),
		namespace.ToNMT(),
		leaves,
		rowRoot,
	)
}
