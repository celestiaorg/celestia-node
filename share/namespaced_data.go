package share

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	types_pb "github.com/celestiaorg/celestia-node/share/pb"
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
		if !row.VerifyInclusion(originalRoots[i], namespace) {
			return fmt.Errorf("row verification failed: row %d doesn't match original root: %s", i, root.String())
		}
	}
	return nil
}

func (row NamespacedRow) Validate(dah *Root, rowIdx int, namespace Namespace) error {
	if len(row.Shares) == 0 && row.Proof.IsEmptyProof() {
		return fmt.Errorf("empty Data")
	}

	rowRoot := dah.RowRoots[rowIdx]
	if namespace.IsOutsideRange(rowRoot, rowRoot) {
		return fmt.Errorf("namespace is outside of the range")
	}

	if !row.VerifyInclusion(rowRoot, namespace) {
		return fmt.Errorf("invalid inclusion proof")
	}
	return nil
}

// VerifyInclusion validates the row using nmt inclusion proof.
func (row NamespacedRow) VerifyInclusion(rowRoot []byte, namespace Namespace) bool {
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

func (row NamespacedRow) ToProto() *types_pb.NamespacedRow {
	return &types_pb.NamespacedRow{
		Shares: row.Shares,
		Proof: &nmt_pb.Proof{
			Start:                 int64(row.Proof.Start()),
			End:                   int64(row.Proof.End()),
			Nodes:                 row.Proof.Nodes(),
			LeafHash:              row.Proof.LeafHash(),
			IsMaxNamespaceIgnored: row.Proof.IsMaxNamespaceIDIgnored(),
		},
	}
}

// NewNamespacedSharesFromEDS samples the EDS and constructs Data for each row with the given namespace.
func NewNamespacedSharesFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	namespace Namespace,
) (NamespacedShares, error) {
	root, err := NewRoot(square)
	if err != nil {
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	var rows NamespacedShares //nolint:prealloc// we don't know how many rows with needed namespace there are
	for rowIdx, rowRoot := range root.RowRoots {
		if namespace.IsOutsideRange(rowRoot, rowRoot) {
			continue
		}

		shrs := square.Row(uint(rowIdx))
		row, err := NamespacedRowFromShares(shrs, namespace, rowIdx)
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, nil
}

func NamespacedRowFromShares(row []Share, namespace Namespace, axisIdx int) (NamespacedRow, error) {
	var from, amount int
	for i := range len(row) / 2 {
		if namespace.Equals(GetNamespace(row[i])) {
			if amount == 0 {
				from = i
			}
			amount++
		}
	}
	if amount == 0 {
		return NamespacedRow{}, fmt.Errorf("no shares in namespace")
	}

	namespacedShares := make([][]byte, amount)
	copy(namespacedShares, row[from:from+amount])

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(row)/2), uint(axisIdx))
	for _, shr := range row {
		err := tree.Push(shr)
		if err != nil {
			return NamespacedRow{}, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	proof, err := tree.ProveRange(from, from+amount)
	if err != nil {
		return NamespacedRow{}, fmt.Errorf("while proving range over NMT: %w", err)
	}

	return NamespacedRow{
		Shares: namespacedShares,
		Proof:  &proof,
	}, nil
}

func NamespacedRowFromProto(row *types_pb.NamespacedRow) NamespacedRow {
	proof := nmt.NewInclusionProof(
		int(row.Proof.Start),
		int(row.Proof.End),
		row.Proof.Nodes,
		row.Proof.IsMaxNamespaceIgnored,
	)
	return NamespacedRow{
		Shares: row.Shares,
		Proof:  &proof,
	}
}
