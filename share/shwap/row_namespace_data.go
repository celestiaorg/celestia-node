package shwap

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// ErrNamespaceOutsideRange is returned by RowNamespaceDataFromShares when the target namespace is
// outside of the namespace range for the given row. In this case, the implementation cannot return
// the non-inclusion proof and will return ErrNamespaceOutsideRange.
var ErrNamespaceOutsideRange = errors.New("target namespace is outside of namespace range for the given root")

// RowNamespaceData holds shares and their corresponding proof for a single row within a namespace.
type RowNamespaceData struct {
	Shares []share.Share `json:"shares"` // Shares within the namespace.
	Proof  *nmt.Proof    `json:"proof"`  // Proof of the shares' inclusion in the namespace.
}

// RowNamespaceDataFromShares extracts and constructs a RowNamespaceData from shares within the
// specified namespace.
func RowNamespaceDataFromShares(
	shares []share.Share,
	namespace share.Namespace,
	rowIndex int,
) (RowNamespaceData, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), uint(rowIndex))
	nmtTree := nmt.New(
		appconsts.NewBaseHashFunc(),
		nmt.NamespaceIDSize(appconsts.NamespaceSize),
		nmt.IgnoreMaxNamespace(true),
	)
	tree.SetTree(nmtTree)

	for _, shr := range shares {
		if err := tree.Push(shr); err != nil {
			return RowNamespaceData{}, fmt.Errorf("failed to build tree for row %d: %w", rowIndex, err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return RowNamespaceData{}, fmt.Errorf("failed to get root for row %d: %w", rowIndex, err)
	}
	if namespace.IsOutsideRange(root, root) {
		return RowNamespaceData{}, ErrNamespaceOutsideRange
	}

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

	// if count is 0, then the namespace is not present in the shares. Return non-inclusion proof.
	if count == 0 {
		proof, err := nmtTree.ProveNamespace(namespace.ToNMT())
		if err != nil {
			return RowNamespaceData{}, fmt.Errorf("failed to generate non-inclusion proof for row %d: %w", rowIndex, err)
		}

		return RowNamespaceData{
			Proof: &proof,
		}, nil
	}

	namespacedShares := make([]share.Share, count)
	copy(namespacedShares, shares[from:from+count])

	proof, err := tree.ProveRange(from, from+count)
	if err != nil {
		return RowNamespaceData{}, fmt.Errorf("failed to generate proof for row %d: %w", rowIndex, err)
	}

	return RowNamespaceData{
		Shares: namespacedShares,
		Proof:  &proof,
	}, nil
}

// RowNamespaceDataFromProto constructs RowNamespaceData out of its protobuf representation.
func RowNamespaceDataFromProto(row *pb.RowNamespaceData) RowNamespaceData {
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

// Validate checks validity of the RowNamespaceData against the Root, Namespace and Row index.
func (rnd RowNamespaceData) Validate(dah *share.Root, namespace share.Namespace, rowIdx int) error {
	if rnd.Proof == nil || rnd.Proof.IsEmptyProof() {
		return fmt.Errorf("nil proof")
	}
	if len(rnd.Shares) == 0 && !rnd.Proof.IsOfAbsence() {
		return fmt.Errorf("empty shares with non-absence proof for row %d", rowIdx)
	}

	if len(rnd.Shares) > 0 && rnd.Proof.IsOfAbsence() {
		return fmt.Errorf("non-empty shares with absence proof for row %d", rowIdx)
	}

	if err := ValidateShares(rnd.Shares); err != nil {
		return fmt.Errorf("invalid shares: %w", err)
	}

	rowRoot := dah.RowRoots[rowIdx]
	if namespace.IsOutsideRange(rowRoot, rowRoot) {
		return fmt.Errorf("namespace out of range for row %d", rowIdx)
	}

	if !rnd.verifyInclusion(rowRoot, namespace) {
		return fmt.Errorf("%w for row: %d", ErrFailedVerification, rowIdx)
	}
	return nil
}

// verifyInclusion checks the inclusion of the row's shares in the provided root using NMT.
func (rnd RowNamespaceData) verifyInclusion(rowRoot []byte, namespace share.Namespace) bool {
	leaves := make([][]byte, 0, len(rnd.Shares))
	for _, sh := range rnd.Shares {
		namespaceBytes := share.GetNamespace(sh)
		leave := make([]byte, len(sh)+len(namespaceBytes))
		copy(leave, namespaceBytes)
		copy(leave[len(namespaceBytes):], sh)
		leaves = append(leaves, leave)
	}

	return rnd.Proof.VerifyNamespace(
		share.NewSHA256Hasher(),
		namespace.ToNMT(),
		leaves,
		rowRoot,
	)
}
