package shwap

import (
	"encoding/json"
	"fmt"

	"github.com/cometbft/cometbft/crypto/merkle"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	nmt_ns "github.com/celestiaorg/nmt/namespace"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// Proof encapsulates the cryptographic evidence required to verify that a set of data shares
// belonging to a specific namespace is correctly included in a committed data root.
//
// A Proof consists of two layers:
//
//  1. sharesProof (NMT Proof):
//     - Proves that a set of shares belonging to a namespace exists within a specific row
//     of the Extended Data Square (EDS).
//     - This is a Namespaced Merkle Tree (NMT) proof which validates both existence and
//     namespace membership within the row.
//
//  2. rowRootProof (Merkle Proof):
//     - Proves that the root of the row (produced by the NMT) is included in the final
//     data root (Data Availability Header / DAH) via a Merkle inclusion proof.
//
// Together, these proofs provide end-to-end verifiability: from the share content, to the row,
// to the final committed root.
//
// Usage Notes:
//   - For complete rows (i.e., where the entire row's shares are included), the `sharesProof`
//     may be omitted or left empty initially, and is typically reconstructed at verification time.
//   - For incomplete rows (e.g., partial start or end), the `sharesProof` must be explicitly
//     constructed and included to validate inclusion.
//   - The rowRootProof must always be present to anchor the row to the data root.
//
// This structure is used in RangeNamespaceData and related types to prove namespace-scoped
// inclusion across one or more rows of the EDS.
type Proof struct {
	sharesProof  *nmt.Proof
	rowRootProof *merkle.Proof
}

func NewProof(shrsProof *nmt.Proof, rowRootProof *merkle.Proof) *Proof {
	return &Proof{sharesProof: shrsProof, rowRootProof: rowRootProof}
}

// SharesProof returns a shares inclusion proof.
func (p *Proof) SharesProof() *nmt.Proof {
	return p.sharesProof
}

// RowRootProof returns a merkle proof of the row root to the data root.
func (p *Proof) RowRootProof() *merkle.Proof {
	return p.rowRootProof
}

// VerifyInclusion verifies the inclusion of the shares to the data root.
func (p *Proof) VerifyInclusion(shares []libshare.Share, namespace libshare.Namespace, dataRoot []byte) error {
	if p.IsEmptyProof() {
		return fmt.Errorf("proof is empty")
	}
	nid := namespace.Bytes()
	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(nid).Size(),
		p.sharesProof.IsMaxNamespaceIDIgnored(),
	)

	leaves := libshare.ToBytes(shares)

	// Compute leaf hashes
	hashes, err := nmt.ComputePrefixedLeafHashes(nth, nid, leaves)
	if err != nil {
		return fmt.Errorf("failed to compute leaf hashes: %w", err)
	}

	root, err := p.sharesProof.ComputeRootWithBasicValidation(nth, nid, hashes, false)
	if err != nil {
		return fmt.Errorf("failed to compute proof root: %w", err)
	}

	// Check if namespace is outside the range
	outside, err := share.IsOutsideRange(namespace, root, root)
	if err != nil {
		return fmt.Errorf("failed to check namespace range: %w", err)
	}
	if outside {
		return fmt.Errorf("namespace %x is outside the root range", namespace.ID())
	}

	// Verify against the data root
	if err := p.rowRootProof.Verify(dataRoot, root); err != nil {
		return fmt.Errorf("row root proof verification failed: %w", err)
	}

	return nil
}

// VerifyNamespace verifies that the provided shares belong to the namespace and match the given data root.
func (p *Proof) VerifyNamespace(shares []libshare.Share, namespace libshare.Namespace, dataRoot []byte) error {
	if p.IsEmptyProof() {
		return fmt.Errorf("proof is empty")
	}
	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(namespace.Bytes()).Size(),
		p.sharesProof.IsMaxNamespaceIDIgnored(),
	)
	nid := namespace.ID()

	// Prepare namespaced leaves
	leaves := make([][]byte, len(shares))
	for i, sh := range shares {
		namespaceBytes := sh.Namespace().Bytes()
		leafBytes := sh.ToBytes()
		leaf := make([]byte, len(namespaceBytes)+len(leafBytes))
		copy(leaf, namespaceBytes)
		copy(leaf[len(namespaceBytes):], leafBytes)
		leaves[i] = leaf
	}

	// Compute or prepare leaf hashes
	var hashes [][]byte
	var err error
	if p.IsOfAbsence() {
		hashes = [][]byte{p.sharesProof.LeafHash()}
	} else {
		hashes, err = nmt.ComputeAndValidateLeafHashes(nth, nid, leaves)
		if err != nil {
			return fmt.Errorf("failed to compute leaf hashes: %w", err)
		}
	}

	root, err := p.sharesProof.ComputeRootWithBasicValidation(nth, nid, hashes, true)
	if err != nil {
		return fmt.Errorf("failed to compute proof root: %w", err)
	}

	// Validate namespace is within the root range
	outside, err := share.IsOutsideRange(namespace, root, root)
	if err != nil {
		return fmt.Errorf("failed to check namespace range: %w", err)
	}
	if outside {
		return fmt.Errorf("namespace %x is outside root range", nid)
	}

	// Verify row root proof against the data root
	if err := p.rowRootProof.Verify(dataRoot, root); err != nil {
		return fmt.Errorf("row root proof verification failed: %w", err)
	}

	return nil
}

// Start returns that start index of the proof
func (p *Proof) Start() int {
	return p.sharesProof.Start()
}

// End returns an *inclusive* end index of the proof.
func (p *Proof) End() int {
	return p.sharesProof.End() - 1
}

func (p *Proof) IsOfAbsence() bool {
	return p.sharesProof.IsOfAbsence()
}

func (p *Proof) MarshalJSON() ([]byte, error) {
	temp := struct {
		ShareProof   *nmt.Proof    `json:"share_proof"`
		RowRootProof *merkle.Proof `json:"row_root_proof"`
	}{
		ShareProof:   p.sharesProof,
		RowRootProof: p.rowRootProof,
	}
	return json.Marshal(temp)
}

func (p *Proof) UnmarshalJSON(data []byte) error {
	temp := struct {
		ShareProof   *nmt.Proof    `json:"share_proof"`
		RowRootProof *merkle.Proof `json:"row_root_proof"`
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	p.sharesProof = temp.ShareProof
	p.rowRootProof = temp.RowRootProof
	return nil
}

func (p *Proof) ToProto() *pb.Proof {
	nmtProof := &nmt_pb.Proof{
		Start:                 int64(p.sharesProof.Start()),
		End:                   int64(p.sharesProof.End()),
		Nodes:                 p.sharesProof.Nodes(),
		LeafHash:              p.sharesProof.LeafHash(),
		IsMaxNamespaceIgnored: p.sharesProof.IsMaxNamespaceIDIgnored(),
	}

	rowRootProofs := &pb.RowRootProof{
		Total:    p.rowRootProof.Total,
		Index:    p.rowRootProof.Index,
		LeafHash: p.rowRootProof.LeafHash,
		Aunts:    p.rowRootProof.Aunts,
	}

	return &pb.Proof{
		SharesProof:  nmtProof,
		RowRootProof: rowRootProofs,
	}
}

func ProofFromProto(p *pb.Proof) (*Proof, error) {
	var proof nmt.Proof
	if p.GetSharesProof().GetLeafHash() != nil {
		proof = nmt.NewAbsenceProof(
			int(p.GetSharesProof().GetStart()),
			int(p.GetSharesProof().GetEnd()),
			p.GetSharesProof().GetNodes(),
			p.GetSharesProof().GetLeafHash(),
			p.GetSharesProof().GetIsMaxNamespaceIgnored(),
		)
	} else {
		proof = nmt.NewInclusionProof(
			int(p.GetSharesProof().GetStart()),
			int(p.GetSharesProof().GetEnd()),
			p.GetSharesProof().GetNodes(),
			p.GetSharesProof().GetIsMaxNamespaceIgnored(),
		)
	}

	rowRootProof := &merkle.Proof{
		Total:    p.GetRowRootProof().GetTotal(),
		Index:    p.GetRowRootProof().GetIndex(),
		LeafHash: p.GetRowRootProof().GetLeafHash(),
		Aunts:    p.GetRowRootProof().GetAunts(),
	}

	return &Proof{
		sharesProof:  &proof,
		rowRootProof: rowRootProof,
	}, nil
}

// IsEmptyProof checks if the proof is empty or not. It returns *false*
// if at least one of the condition is not met.
func (p *Proof) IsEmptyProof() bool {
	return p == nil ||
		p.sharesProof == nil ||
		p.sharesProof.IsEmptyProof() ||
		p.rowRootProof == nil
}
