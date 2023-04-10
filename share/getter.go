package share

import (
	"context"
	"fmt"

	"github.com/minio/sha256-simd"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/celestiaorg/rsmt2d"
)

// Getter interface provides a set of accessors for shares by the Root.
// Automatically verifies integrity of shares(exceptions possible depending on the implementation).
//
//go:generate mockgen -destination=mocks/getter.go -package=mocks . Getter
type Getter interface {
	// GetShare gets a Share by coordinates in EDS.
	GetShare(ctx context.Context, root *Root, row, col int) (Share, error)

	// GetEDS gets the full EDS identified by the given root.
	GetEDS(context.Context, *Root) (*rsmt2d.ExtendedDataSquare, error)

	// GetSharesByNamespace gets all shares from an EDS within the given namespace.
	// Shares are returned in a row-by-row order if the namespace spans multiple rows.
	GetSharesByNamespace(context.Context, *Root, namespace.ID) (NamespacedShares, error)
}

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
	Shares []Share
	Proof  *nmt.Proof
}

// Verify validates NamespacedShares by checking every row with nmt inclusion proof.
func (ns NamespacedShares) Verify(root *Root, nID namespace.ID) error {
	originalRoots := make([][]byte, 0)
	rowRanges := getRowRanges(root.RowsRoots, nID.Size())
	for i, rowRange := range rowRanges {
		if !nID.Less(rowRange.min) && nID.LessOrEqual(rowRange.max) {
			originalRoots = append(originalRoots, root.RowsRoots[i])
		}
	}

	if len(originalRoots) != len(ns) {
		return fmt.Errorf("amount of rows differs between root and namespace shares: expected %d, got %d",
			len(originalRoots), len(ns))
	}

	for i, row := range ns {
		// verify row data against row hash from original root
		if !row.verify(originalRoots[i], nID) {
			return fmt.Errorf("row verification failed: row %d doesn't match original root: %s", i, root.Hash())
		}
	}
	return nil
}

// rowRange is a range of namespaces spanned by a single row of the ODS.
type rowRange struct {
	// min is the minimum namespace for the row
	min []byte
	// max is the maximum namespace for the row
	max []byte
}

// getRowRanges returns the range of namespaces spanned by each row of the
// original data square (ODS). The RowsRoots parameter is a list of NMT row roots
// where one row root represents one row of the extended data square (EDS). The
// order of RowsRoots is preserved in the result. This function is necessary
// because the max namespace for all EDS rows will be the parity namespace so we
// use a property of the square layout to determine the max namespace for an ODS
// row: the max namespace for an ODS row is < the min namespace for the next EDS
// row.
func getRowRanges(RowsRoots [][]byte, nidSize namespace.IDSize) (result []rowRange) {
	for i, root := range RowsRoots {
		min := nmt.MinNamespace(root, nidSize)
		var max []byte
		if i == len(RowsRoots)-1 {
			// if this is the last row, the max namespace is the max namespace of the EDS row
			max = nmt.MaxNamespace(root, nidSize)
		} else {
			// if this is not the last row, the namespace is the min namespace of the next EDS row
			max = nmt.MinNamespace(RowsRoots[i+1], nidSize)
		}
		result = append(result, rowRange{min: min, max: max})
	}
	return result
}

// verify validates the row using nmt inclusion proof.
func (row *NamespacedRow) verify(rowRoot []byte, nID namespace.ID) bool {
	// construct nmt leaves from shares by prepending namespace
	leaves := make([][]byte, 0, len(row.Shares))
	for _, sh := range row.Shares {
		leaves = append(leaves, append(sh[:NamespaceSize], sh...))
	}

	// verify namespace
	return row.Proof.VerifyNamespace(
		sha256.New(),
		nID,
		leaves,
		rowRoot)
}
