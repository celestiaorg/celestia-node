package shwap

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/shwap/proto"
)

// NamespacedData stores collections of RowNamespaceData, each representing shares and their proofs
// within a namespace.
type NamespacedData []RowNamespaceData

// Flatten combines all shares from all rows within the namespace into a single slice.
func (ns NamespacedData) Flatten() []share.Share {
	var shares []share.Share
	for _, row := range ns {
		shares = append(shares, row.Shares...)
	}
	return shares
}

// RowNamespaceData holds shares and their corresponding proof for a single row within a namespace.
type RowNamespaceData struct {
	Shares []share.Share `json:"shares"` // Shares within the namespace.
	Proof  *nmt.Proof    `json:"proof"`  // Proof of the shares' inclusion in the namespace.
}

// Verify checks the integrity of the NamespacedData against a provided root and namespace.
func (ns NamespacedData) Verify(root *share.Root, namespace share.Namespace) error {
	var originalRoots [][]byte
	for _, rowRoot := range root.RowRoots {
		if !namespace.IsOutsideRange(rowRoot, rowRoot) {
			originalRoots = append(originalRoots, rowRoot)
		}
	}

	if len(originalRoots) != len(ns) {
		return fmt.Errorf("expected %d rows, found %d rows", len(originalRoots), len(ns))
	}

	for i, row := range ns {
		if row.Proof == nil || len(row.Shares) == 0 {
			return fmt.Errorf("row %d is missing proofs or shares", i)
		}
		if !row.VerifyInclusion(originalRoots[i], namespace) {
			return fmt.Errorf("failed to verify row %d", i)
		}
	}
	return nil
}

func (rnd RowNamespaceData) Validate(dah *share.Root, rowIdx int, namespace share.Namespace) error {
	if len(rnd.Shares) == 0 && rnd.Proof.IsEmptyProof() {
		return fmt.Errorf("row contains no data or proof")
	}

	rowRoot := dah.RowRoots[rowIdx]
	if namespace.IsOutsideRange(rowRoot, rowRoot) {
		return fmt.Errorf("namespace out of range for row %d", rowIdx)
	}

	if !rnd.VerifyInclusion(rowRoot, namespace) {
		return fmt.Errorf("inclusion proof failed for row %d", rowIdx)
	}
	return nil
}

// VerifyInclusion checks the inclusion of the row's shares in the provided root using NMT.
func (rnd RowNamespaceData) VerifyInclusion(rowRoot []byte, namespace share.Namespace) bool {
	leaves := make([][]byte, 0, len(rnd.Shares))
	for _, shr := range rnd.Shares {
		namespaceBytes := share.GetNamespace(shr)
		leaves = append(leaves, append(namespaceBytes, shr...))
	}
	return rnd.Proof.VerifyNamespace(
		sha256.New(),
		namespace.ToNMT(),
		leaves,
		rowRoot,
	)
}

// ToProto converts RowNamespaceData to its protobuf representation for serialization.
func (rnd RowNamespaceData) ToProto() *pb.RowNamespaceData {
	return &pb.RowNamespaceData{
		Shares: SharesToProto(rnd.Shares),
		Proof: &nmt_pb.Proof{
			Start:                 int64(rnd.Proof.Start()),
			End:                   int64(rnd.Proof.End()),
			Nodes:                 rnd.Proof.Nodes(),
			LeafHash:              rnd.Proof.LeafHash(),
			IsMaxNamespaceIgnored: rnd.Proof.IsMaxNamespaceIDIgnored(),
		},
	}
}

// NewNamespacedSharesFromEDS extracts shares for a specific namespace from an EDS, considering
// each row independently.
func NewNamespacedSharesFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	namespace share.Namespace,
) (NamespacedData, error) {
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, fmt.Errorf("error computing root: %w", err)
	}

	rows := make(NamespacedData, 0, len(root.RowRoots))
	for rowIdx, rowRoot := range root.RowRoots {
		if namespace.IsOutsideRange(rowRoot, rowRoot) {
			continue
		}

		shares := square.Row(uint(rowIdx))
		rowData, err := NamespacedRowFromShares(shares, namespace, rowIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to process row %d: %w", rowIdx, err)
		}

		rows = append(rows, rowData)
	}

	return rows, nil
}

// NamespacedRowFromShares extracts and constructs a RowNamespaceData from shares within the
// specified namespace.
func NamespacedRowFromShares(shares []share.Share, namespace share.Namespace, rowIndex int) (RowNamespaceData, error) {
	var from, count int
	for i := range len(shares) / 2 {
		if namespace.Equals(share.GetNamespace(shares[i])) {
			if count == 0 {
				from = i
			}
			count++
			continue
		}
		if count > 0 {
			break
		}
	}
	if count == 0 {
		return RowNamespaceData{}, fmt.Errorf("no shares found in the namespace for row %d", rowIndex)
	}

	namespacedShares := make([]share.Share, count)
	copy(namespacedShares, shares[from:from+count])

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), uint(rowIndex))
	for _, shr := range shares {
		if err := tree.Push(shr); err != nil {
			return RowNamespaceData{}, fmt.Errorf("failed to build tree for row %d: %w", rowIndex, err)
		}
	}

	proof, err := tree.ProveRange(from, from+count)
	if err != nil {
		return RowNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", rowIndex, err)
	}

	return RowNamespaceData{
		Shares: namespacedShares,
		Proof:  &proof,
	}, nil
}

func NamespacedRowFromProto(row *pb.RowNamespaceData) RowNamespaceData {
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
