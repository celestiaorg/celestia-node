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

// Proof represents a proof that a data share is included in a
// committed data root.
// It consists of multiple components to support verification:
// * shareProof: A nmt proof that verifies the inclusion of a specific data share within a row of the datasquare.
// * rowRootProof: A Merkle proof that verifies the inclusion of the row root within the final data root.
type Proof struct {
	shareProof   *nmt.Proof
	rowRootProof *merkle.Proof
}

func NewProof(sharesProofs *nmt.Proof, rowRootProof *merkle.Proof) *Proof {
	return &Proof{shareProof: sharesProofs, rowRootProof: rowRootProof}
}

// SharesProof returns a shares inclusion proof.
func (p *Proof) SharesProof() *nmt.Proof {
	return p.shareProof
}

// RowRootProof returns a merkle proof of the row root to the data root.
func (p *Proof) RowRootProof() *merkle.Proof {
	return p.rowRootProof
}

// VerifyInclusion verifies the inclusion of the shares to the data root.
func (p *Proof) VerifyInclusion(shares []libshare.Share, namespace libshare.Namespace, dataRoot []byte) error {
	nid := namespace.Bytes()
	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(nid).Size(),
		p.shareProof.IsMaxNamespaceIDIgnored(),
	)

	leaves := libshare.ToBytes(shares)

	// Compute leaf hashes
	hashes, err := nmt.ComputePrefixedLeafHashes(nth, nid, leaves)
	if err != nil {
		return fmt.Errorf("failed to compute leaf hashes: %w", err)
	}

	// Validate the proof structure
	if err := p.shareProof.ValidateProofStructure(nth, nid, hashes); err != nil {
		return fmt.Errorf("invalid proof structure: %w", err)
	}

	// Compute the root from proof and leaf hashes
	root, err := p.shareProof.ComputeRoot(nth, hashes)
	if err != nil {
		return fmt.Errorf("failed to compute root from proof: %w", err)
	}

	// Validate the computed root's format
	if err := nth.ValidateNodeFormat(root); err != nil {
		return fmt.Errorf("invalid node format for root: %w", err)
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
	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(namespace.Bytes()).Size(),
		p.shareProof.IsMaxNamespaceIDIgnored(),
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
		hashes = [][]byte{p.shareProof.LeafHash()}
	} else {
		hashes, err = nmt.ComputeAndValidateLeafHashes(nth, nid, leaves)
		if err != nil {
			return fmt.Errorf("failed to compute leaf hashes: %w", err)
		}
	}

	// Validate proof structure
	if err := p.shareProof.ValidateProofStructure(nth, nid, hashes); err != nil {
		return fmt.Errorf("invalid proof structure: %w", err)
	}

	// For inclusion proofs, validate single namespace consistency
	if !p.IsOfAbsence() {
		if err := p.shareProof.ValidateNamespace(nth, nid, hashes); err != nil {
			return fmt.Errorf("invalid namespace consistency: %w", err)
		}
	}

	// Validate completeness (no missed leaves outside proof range)
	if err := p.shareProof.ValidateCompleteness(nth, nid); err != nil {
		return fmt.Errorf("proof completeness failed: %w", err)
	}

	// Reconstruct the root from the proof
	root, err := p.shareProof.ComputeRoot(nth, hashes)
	if err != nil {
		return fmt.Errorf("failed to compute proof root: %w", err)
	}

	// Ensure the reconstructed root is valid
	if err := nth.ValidateNodeFormat(root); err != nil {
		return fmt.Errorf("invalid node format for root: %w", err)
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
	return p.shareProof.Start()
}

// End returns an *inclusive* end index of the proof.
func (p *Proof) End() int {
	return p.shareProof.End() - 1
}

func (p *Proof) IsOfAbsence() bool {
	return p.shareProof.IsOfAbsence()
}

func (p *Proof) MarshalJSON() ([]byte, error) {
	temp := struct {
		ShareProof   *nmt.Proof    `json:"share_proof"`
		RowRootProof *merkle.Proof `json:"row_root_proof"`
	}{
		ShareProof:   p.shareProof,
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
	p.shareProof = temp.ShareProof
	p.rowRootProof = temp.RowRootProof
	return nil
}

func (p *Proof) ToProto() *pb.Proof {
	nmtProof := &nmt_pb.Proof{
		Start:                 int64(p.shareProof.Start()),
		End:                   int64(p.shareProof.End()),
		Nodes:                 p.shareProof.Nodes(),
		LeafHash:              p.shareProof.LeafHash(),
		IsMaxNamespaceIgnored: p.shareProof.IsMaxNamespaceIDIgnored(),
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
		shareProof:   &proof,
		rowRootProof: rowRootProof,
	}, nil
}

// IsEmptyProof checks if the proof is empty or not. It returns *false*
// if at least one of the condition is not met.
func (p *Proof) IsEmptyProof() bool {
	return p == nil ||
		p.shareProof == nil ||
		p.shareProof.IsEmptyProof() ||
		p.rowRootProof == nil
}
