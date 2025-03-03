package shwap

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/crypto/merkle"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// Proof represents a proof that a data share is included in a
// committed data root.
// It consists of multiple components to support verification:
// * shareProof: A nmt proof that verifies the inclusion of a specific data share within a row of the datasquare.
// * rowRootProof: A Merkle proof that verifies the inclusion of the row root within the final data root.
// * root: The row root against which the proof is verified
type Proof struct {
	shareProof   *nmt.Proof
	rowRootProof *merkle.Proof
	root         []byte
}

func NewProof(rowIndex int, sharesProofs *nmt.Proof, root *share.AxisRoots) *Proof {
	proof := &Proof{shareProof: sharesProofs}
	roots := append(root.RowRoots, root.ColumnRoots...) //nolint: gocritic
	_, proofs := merkle.ProofsFromByteSlices(roots)

	proof.rowRootProof = proofs[rowIndex]
	proof.root = roots[rowIndex]
	return proof
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
	outside, err := share.IsOutsideRange(namespace, p.root, p.root)
	if err != nil {
		return err
	}
	if outside {
		return fmt.Errorf("namespace out of range")
	}

	isValid := p.shareProof.VerifyInclusion(
		share.NewSHA256Hasher(),
		namespace.Bytes(),
		libshare.ToBytes(shares),
		p.root,
	)
	if !isValid {
		return errors.New("failed to verify nmt proof")
	}

	err = p.rowRootProof.Verify(dataRoot, p.root)
	if err != nil {
		return err
	}
	return nil
}

// VerifyNamespace verifies the whole namespace of the shares to the data root.
func (p *Proof) VerifyNamespace(shrs []libshare.Share, namespace libshare.Namespace, dataRoot []byte) error {
	outside, err := share.IsOutsideRange(namespace, p.root, p.root)
	if err != nil {
		return err
	}
	if outside {
		return fmt.Errorf("namespace out of range")
	}

	leaves := make([][]byte, 0, len(shrs))
	for _, sh := range shrs {
		namespaceBytes := sh.Namespace().Bytes()
		leave := make([]byte, len(sh.ToBytes())+len(namespaceBytes))
		copy(leave, namespaceBytes)
		copy(leave[len(namespaceBytes):], sh.ToBytes())
		leaves = append(leaves, leave)
	}

	valid := p.shareProof.VerifyNamespace(
		share.NewSHA256Hasher(),
		namespace.Bytes(),
		leaves,
		p.root,
	)
	if !valid {
		return fmt.Errorf("failed to verify namespace for the shares in row")
	}

	err = p.rowRootProof.Verify(dataRoot, p.root)
	if err != nil {
		return err
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
		Roots        []byte        `json:"root"`
	}{
		ShareProof:   p.shareProof,
		RowRootProof: p.rowRootProof,
		Roots:        p.root,
	}
	return json.Marshal(temp)
}

func (p *Proof) UnmarshalJSON(data []byte) error {
	temp := struct {
		ShareProof   *nmt.Proof    `json:"share_proof"`
		RowRootProof *merkle.Proof `json:"row_root_proof"`
		Root         []byte        `json:"root"`
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	p.shareProof = temp.ShareProof
	p.rowRootProof = temp.RowRootProof
	p.root = temp.Root
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
		Root:         p.root,
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
		root:         p.GetRoot(),
	}, nil
}

// IsEmptyProof checks if the proof is empty or not. It returns *false*
// if at least one of the condition is not met.
func (p *Proof) IsEmptyProof() bool {
	return p == nil ||
		p.shareProof == nil ||
		p.shareProof.IsEmptyProof() ||
		p.rowRootProof == nil ||
		p.root == nil
}
