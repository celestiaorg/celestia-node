package proof

import (
	"encoding/json"
	"fmt"

	"github.com/celestiaorg/celestia-app/v4/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	nmt_ns "github.com/celestiaorg/nmt/namespace"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share"
	proof_pb "github.com/celestiaorg/celestia-node/share/proof/pb"
)

type sharesProof = [2]*nmt.Proof

type DataRootProof struct {
	sharesProof  *sharesProof
	rowRootProof *MerkleProof
}

func NewDataRootProof(sharesProof *sharesProof, root *share.AxisRoots, start, end int64) *DataRootProof {
	items := append(root.RowRoots, root.ColumnRoots...) //nolint: gocritic
	proof := NewProof(items, start, end)
	return &DataRootProof{
		sharesProof:  sharesProof,
		rowRootProof: proof,
	}
}

// SharesProof returns a shares inclusion proof.
// The returned value can be nil.
func (p *DataRootProof) SharesProof() *[2]*nmt.Proof {
	return p.sharesProof
}

// RowRootProof returns a merkle proof of the row root to the data root.
func (p *DataRootProof) RowRootProof() *MerkleProof {
	return p.rowRootProof
}

func (p *DataRootProof) VerifyInclusion(shares [][]libshare.Share, dataRootHash []byte) error {
	return p.verify(shares, dataRootHash, false)
}

func (p *DataRootProof) VerifyNamespace(shares [][]libshare.Share, dataRootHash []byte) error {
	return p.verify(shares, dataRootHash, true)
}

func (p *DataRootProof) ToProto() *proof_pb.DataRootProof {
	var pbSharesProof []*nmt_pb.Proof
	if p.sharesProof != nil {
		pbSharesProof = make([]*nmt_pb.Proof, 0)
		for _, sharesProof := range p.sharesProof {
			pbSharesProof = append(pbSharesProof, &nmt_pb.Proof{
				Start:                 int64(sharesProof.Start()),
				End:                   int64(sharesProof.End()),
				Nodes:                 sharesProof.Nodes(),
				LeafHash:              sharesProof.LeafHash(),
				IsMaxNamespaceIgnored: sharesProof.IsMaxNamespaceIDIgnored(),
			})
		}
	}
	return &proof_pb.DataRootProof{
		RowRootProof: p.rowRootProof.ToProto(),
		SharesProof:  pbSharesProof,
	}
}

func DataRootProofFromProto(p *proof_pb.DataRootProof) (*DataRootProof, error) {
	var proofs *[2]*nmt.Proof
	for i, pp := range p.SharesProof {
		if proofs == nil {
			proofs = new([2]*nmt.Proof)
		}
		if pp.GetLeafHash() != nil {
			proof := nmt.NewAbsenceProof(
				int(pp.GetStart()),
				int(pp.GetEnd()),
				pp.GetNodes(),
				pp.GetLeafHash(),
				pp.GetIsMaxNamespaceIgnored(),
			)
			proofs[i] = &proof
		} else {
			proof := nmt.NewInclusionProof(
				int(pp.GetStart()),
				int(pp.GetEnd()),
				pp.GetNodes(),
				pp.GetIsMaxNamespaceIgnored(),
			)
			proofs[i] = &proof
		}
	}

	return &DataRootProof{
		sharesProof:  proofs,
		rowRootProof: MerkleProofFromProto(p.RowRootProof),
	}, nil
}

// verify validates the DataRootProof against the provided shares and data root hash.
// It reconstructs row roots from the shares and verifies them against the data root.
//
// Parameters:
//   - shares: a slice of shares organized by rows
//   - dataRootHash: the expected root hash of the entire data square
//   - verifyNsCompleteness: whether to verify namespace completeness in proofs
func (p *DataRootProof) verify(shares [][]libshare.Share, dataRootHash []byte, verifyNsCompleteness bool) error {
	if p.rowRootProof == nil {
		return fmt.Errorf("row root proof is empty")
	}

	if len(shares) == 0 || len(shares[0]) == 0 {
		return fmt.Errorf("empty shares provided")
	}

	// Verify the number of row shares matches the expected range from the proof
	if int64(len(shares)) != p.rowRootProof.End-p.rowRootProof.Start {
		return fmt.Errorf("incorrect number of row shares provided")
	}

	namespace := shares[0][0].Namespace().Bytes()
	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(namespace).Size(),
		true,
	)

	// Initialize array to store computed row roots for each row of shares
	rowRoots := make([][]byte, len(shares))

	// Process rows that have proofs
	// These are typically incomplete rows (partial namespace data within a row)
	for _, proof := range p.sharesProof {
		if proof == nil {
			break // No more proofs to process
		}

		nth.Reset()

		var (
			leaves [][]byte // The actual share data to hash
			index  uint     // Index of the row being processed
		)

		// Determine which row's shares to use based on proof structure
		if proof.Start() > 0 {
			// Incomplete row at proof start - use first row's shares
			leaves = libshare.ToBytes(shares[0])
		} else {
			// Incomplete row at proof end - use last row's shares
			leaves = libshare.ToBytes(shares[len(shares)-1])
			index = uint(len(shares) - 1)
		}

		// Compute leaf hashes for the namespace merkle tree
		hashes, err := nmt.ComputePrefixedLeafHashes(nth, namespace, leaves)
		if err != nil {
			return fmt.Errorf("failed to compute leaf hashes: %w", err)
		}

		// Compute the row root using the namespace merkle tree proof
		root, err := proof.ComputeRootWithBasicValidation(nth, namespace, hashes, verifyNsCompleteness)
		if err != nil {
			return fmt.Errorf("failed to compute proof root: %w", err)
		}
		rowRoots[index] = root
	}

	// Handle rows that don't have namespace proofs (complete rows)
	for i := range rowRoots {
		// Skip rows that already have their roots computed from proofs
		if rowRoots[i] != nil {
			continue
		}

		extendedRowShares, err := share.ExtendShares(shares[i])
		if err != nil {
			return fmt.Errorf("failed to extend shares: %w", err)
		}

		// Build the row root from the extended shares
		root, err := buildTreeRootFromLeaves(libshare.ToBytes(extendedRowShares), uint(p.rowRootProof.Start)+uint(i))
		if err != nil {
			return fmt.Errorf("failed to build shares proof: %w", err)
		}

		// Store the computed root for this row
		rowRoots[i] = root
	}

	if !p.rowRootProof.Verify(dataRootHash, rowRoots) {
		return fmt.Errorf("row roots validation failed")
	}
	return nil
}

func (p *DataRootProof) MarshalJSON() ([]byte, error) {
	temp := struct {
		ShareProof   *[2]*nmt.Proof `json:"shares_proof,omitempty"`
		RowRootProof *MerkleProof   `json:"row_root_proof"`
	}{
		ShareProof:   p.sharesProof,
		RowRootProof: p.rowRootProof,
	}
	return json.Marshal(temp)
}

func (p *DataRootProof) UnmarshalJSON(data []byte) error {
	temp := struct {
		ShareProof   *[2]*nmt.Proof `json:"shares_proof"`
		RowRootProof *MerkleProof   `json:"row_root_proof"`
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	p.sharesProof = temp.ShareProof
	p.rowRootProof = temp.RowRootProof
	return nil
}

func buildTreeRootFromLeaves(shares [][]byte, index uint) ([]byte, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), index)
	for _, shr := range shares {
		if err := tree.Push(shr); err != nil {
			return nil, fmt.Errorf("failed to build tree for row %d: %w", index, err)
		}
	}
	return tree.Root()
}
