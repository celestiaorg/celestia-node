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
// * sharesProof: A nmt proof that verifies the inclusion of data shares within a row of the datasquare.
// * rowRootProof: A Merkle proof that verifies the inclusion of the row root within the final data root.
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
