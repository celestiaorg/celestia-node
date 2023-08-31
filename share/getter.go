package share

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"
)

var (
	// ErrNotFound is used to indicate that requested data could not be found.
	ErrNotFound = errors.New("share: data not found")
	// ErrOutOfBounds is used to indicate that a passed row or column index is out of bounds of the
	// square size.
	ErrOutOfBounds = errors.New("share: row or column index is larger than square size")
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
	// Inclusion of returned data could be verified using Verify method on NamespacedShares.
	// If no shares are found for target namespace non-inclusion could be also verified by calling
	// Verify method.
	GetSharesByNamespace(context.Context, *Root, Namespace) (NamespacedShares, error)
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
		// verify row data against row hash from original root
		if !row.verify(originalRoots[i], namespace) {
			return fmt.Errorf("row verification failed: row %d doesn't match original root: %s", i, root.String())
		}
	}
	return nil
}

// verify validates the row using nmt inclusion proof.
func (row *NamespacedRow) verify(rowRoot []byte, namespace Namespace) bool {
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
