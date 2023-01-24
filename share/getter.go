package share

import (
	"context"
	"fmt"

	"github.com/minio/sha256-simd"

	"github.com/celestiaorg/celestia-node/share/ipld"

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
	Proof  *ipld.Proof
}

// Verify validates NamespacedShares by checking every row with nmt inclusion proof.
func (ns NamespacedShares) Verify(root *Root, nID namespace.ID) error {
	originalRoots := make([][]byte, 0)
	for _, row := range root.RowsRoots {
		if !nID.Less(nmt.MinNamespace(row, nID.Size())) && nID.LessOrEqual(nmt.MaxNamespace(row, nID.Size())) {
			originalRoots = append(originalRoots, row)
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

// verify validates the row using nmt inclusion proof.
func (row *NamespacedRow) verify(rowRoot []byte, nID namespace.ID) bool {
	// construct nmt leaves from shares by prepending namespace
	leaves := make([][]byte, 0, len(row.Shares))
	for _, sh := range row.Shares {
		leaves = append(leaves, append(sh[:NamespaceSize], sh...))
	}

	proofNodes := make([][]byte, 0, len(row.Proof.Nodes))
	for _, n := range row.Proof.Nodes {
		proofNodes = append(proofNodes, ipld.NamespacedSha256FromCID(n))
	}

	// construct new proof
	inclusionProof := nmt.NewInclusionProof(
		row.Proof.Start,
		row.Proof.End,
		proofNodes,
		ipld.NMTIgnoreMaxNamespace)

	// verify inclusion
	return inclusionProof.VerifyNamespace(
		sha256.New(),
		nID,
		leaves,
		rowRoot)
}
