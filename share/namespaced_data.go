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

// NamespacedData represents all shares with proofs within a specific namespace of an EDS.
type NamespacedData []RowNamespaceData

// Flatten returns the concatenated slice of all RowNamespaceData shares.
func (ns NamespacedData) Flatten() []Share {
	shares := make([]Share, 0)
	for _, row := range ns {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// RowNamespaceData represents all shares with proofs within a specific namespace of a single EDS row.
type RowNamespaceData struct {
	Shares []Share    `json:"shares"`
	Proof  *nmt.Proof `json:"proof"`
}

// Verify validates NamespacedData by checking every row with nmt inclusion proof.
func (ns NamespacedData) Verify(root *Root, namespace Namespace) error {
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

func (rnd RowNamespaceData) Validate(dah *Root, rowIdx int, namespace Namespace) error {
	if len(rnd.Shares) == 0 && rnd.Proof.IsEmptyProof() {
		return fmt.Errorf("empty Data")
	}

	rowRoot := dah.RowRoots[rowIdx]
	if namespace.IsOutsideRange(rowRoot, rowRoot) {
		return fmt.Errorf("namespace is outside of the range")
	}

	if !rnd.VerifyInclusion(rowRoot, namespace) {
		return fmt.Errorf("invalid inclusion proof")
	}
	return nil
}

// VerifyInclusion validates the row using nmt inclusion proof.
func (rnd RowNamespaceData) VerifyInclusion(rowRoot []byte, namespace Namespace) bool {
	// construct nmt leaves from shares by prepending namespace
	leaves := make([][]byte, 0, len(rnd.Shares))
	for _, shr := range rnd.Shares {
		leaves = append(leaves, append(GetNamespace(shr), shr...))
	}
	// verify namespace
	return rnd.Proof.VerifyNamespace(
		sha256.New(),
		namespace.ToNMT(),
		leaves,
		rowRoot,
	)
}

func (rnd RowNamespaceData) ToProto() *types_pb.RowNamespaceData {
	return &types_pb.RowNamespaceData{
		Shares: Shares(rnd.Shares).ToProto(),
		Proof: &nmt_pb.Proof{
			Start:                 int64(rnd.Proof.Start()),
			End:                   int64(rnd.Proof.End()),
			Nodes:                 rnd.Proof.Nodes(),
			LeafHash:              rnd.Proof.LeafHash(),
			IsMaxNamespaceIgnored: rnd.Proof.IsMaxNamespaceIDIgnored(),
		},
	}
}

// NewNamespacedSharesFromEDS samples the EDS and constructs Data for each row with the given namespace.
func NewNamespacedSharesFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	namespace Namespace,
) (NamespacedData, error) {
	root, err := NewRoot(square)
	if err != nil {
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	var rows NamespacedData //nolint:prealloc// we don't know how many rows with needed namespace there are
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

func NamespacedRowFromShares(row []Share, namespace Namespace, axisIdx int) (RowNamespaceData, error) {
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
		return RowNamespaceData{}, fmt.Errorf("no shares in namespace")
	}

	namespacedShares := make([][]byte, amount)
	copy(namespacedShares, row[from:from+amount])

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(row)/2), uint(axisIdx))
	for _, shr := range row {
		err := tree.Push(shr)
		if err != nil {
			return RowNamespaceData{}, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	proof, err := tree.ProveRange(from, from+amount)
	if err != nil {
		return RowNamespaceData{}, fmt.Errorf("while proving range over NMT: %w", err)
	}

	return RowNamespaceData{
		Shares: namespacedShares,
		Proof:  &proof,
	}, nil
}

func NamespacedRowFromProto(row *types_pb.RowNamespaceData) RowNamespaceData {
	var proof nmt.Proof
	if row.GetProof().GetLeafHash() != nil {
		proof = nmt.NewAbsenceProof(
			int(row.GetProof().GetStart()),
			int(row.GetProof().GetEnd()),
			row.GetProof().GetNodes(),
			row.GetProof().GetLeafHash(),
			row.GetProof().GetIsMaxNamespaceIgnored(),
		)
	} else {
		proof = nmt.NewInclusionProof(
			int(row.GetProof().GetStart()),
			int(row.GetProof().GetEnd()),
			row.GetProof().GetNodes(),
			row.GetProof().GetIsMaxNamespaceIgnored(),
		)

	}

	return RowNamespaceData{
		Shares: SharesFromProto(row.GetShares()),
		Proof:  &proof,
	}
}
