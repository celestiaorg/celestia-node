package proof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-app/v4/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/nmt"
	nmt_ns "github.com/celestiaorg/nmt/namespace"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	proof_pb "github.com/celestiaorg/celestia-node/share/proof/pb"
)

type DataRootProof struct {
	// leftSharesProof provides inclusion proof for the first row(of the range) if it's incomplete
	// (i.e., the namespace doesn't span the entire row). nil if the row is complete.
	leftSharesProof *nmt.Proof
	// rightSharesProof provides namespace proof for the last row if it's incomplete
	// (i.e., the namespace doesn't span the entire row). nil if the row is complete.
	rightSharesProof *nmt.Proof
	// rowRootProof provides Merkle inclusion proof for the row roots within the data root.
	// nil if range spans the whole ods.
	rowRootProof *MerkleProof
}

func NewDataRootProof(leftProof, rightProof *nmt.Proof, root *share.AxisRoots, start, end int64) *DataRootProof {
	dataRootProof := &DataRootProof{
		leftSharesProof:  leftProof,
		rightSharesProof: rightProof,
	}

	if start != 0 || end != int64(len(root.RowRoots)/2) ||
		leftProof != nil || rightProof != nil {
		items := append(root.RowRoots, root.ColumnRoots...) //nolint: gocritic
		dataRootProof.rowRootProof = NewProof(items, start, end)
	} else {
		dataRootProof.rowRootProof = &MerkleProof{
			Start: 0,
			End:   int64(len(root.RowRoots) / 2),
			Total: int64(len(root.RowRoots)) * 2,
		}
	}
	return dataRootProof
}

// Start returns the ods index of the first share of the range.
func (p *DataRootProof) Start() int64 {
	odsSize := p.rowRootProof.Total / 4
	startRow := p.rowRootProof.Start
	startCol := 0
	if p.leftSharesProof != nil {
		startCol = p.leftSharesProof.Start()
	}
	return startRow*odsSize + int64(startCol)
}

// End returns the end index of the last share of the range.
func (p *DataRootProof) End() int64 {
	odsSize := p.rowRootProof.Total / 4
	endRow := p.rowRootProof.End - 1
	endCol := odsSize - 1
	if p.rightSharesProof != nil {
		endCol = int64(p.rightSharesProof.End() - 1)
	} else if p.leftSharesProof != nil { // in case we have a single row range
		endCol = int64(p.leftSharesProof.End() - 1)
	}
	return endRow*odsSize + endCol
}

func (p *DataRootProof) ODSSize() int64 {
	return p.rowRootProof.Total / 4
}

// LeftProof returns the proof of the first row.
// NOTE: can be nil
func (p *DataRootProof) LeftProof() *nmt.Proof {
	return p.leftSharesProof
}

// RightProof returns the proof of the last row.
// NOTE: can be nil
func (p *DataRootProof) RightProof() *nmt.Proof {
	return p.rightSharesProof
}

// RowRootProof returns a merkle proof of the row root to the data root.
// NOTE: can be nil
func (p *DataRootProof) RowRootProof() *MerkleProof {
	return p.rowRootProof
}

// VerifyInclusion verifies that the provided shares are included in the data root.
func (p *DataRootProof) VerifyInclusion(shares [][]libshare.Share, dataRootHash []byte) error {
	return p.verify(shares, dataRootHash, false)
}

// VerifyNamespace verifies that the provided shares are included in the data root with a specific
// namespace validation.
func (p *DataRootProof) VerifyNamespace(shares [][]libshare.Share, dataRootHash []byte) error {
	return p.verify(shares, dataRootHash, true)
}

func (p *DataRootProof) ToProto() *proof_pb.DataRootProof {
	return &proof_pb.DataRootProof{
		RowRootProof:     p.rowRootProof.ToProto(),
		LeftSharesProof:  nmtToNmtPbProof(p.leftSharesProof),
		RightSharesProof: nmtToNmtPbProof(p.rightSharesProof),
	}
}

func DataRootProofFromProto(p *proof_pb.DataRootProof) (*DataRootProof, error) {
	return &DataRootProof{
		leftSharesProof:  pbNmtTonmtProof(p.LeftSharesProof),
		rightSharesProof: pbNmtTonmtProof(p.RightSharesProof),
		rowRootProof:     MerkleProofFromProto(p.RowRootProof),
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
	namespace := shares[0][0].Namespace()
	for _, rowShare := range shares {
		for _, share := range rowShare {
			if !namespace.Equals(share.Namespace()) {
				return errors.New("namespace mismatch")
			}
		}
	}

	// verify special case when the requested range spans across the whole eds.
	if p.rowRootProof.SubtreeRoots == nil && p.leftSharesProof == nil && p.rightSharesProof == nil {
		// reconstruct the axis roots
		roots, err := reconstructEDS(shares)
		if err != nil {
			return fmt.Errorf("failed to build the eds to verify the proof")
		}
		// compare hashes
		if !bytes.Equal(roots.Hash(), dataRootHash) {
			return errors.New("data root hash mismatch")
		}
		return nil
	}

	if len(shares) == 0 || len(shares[0]) == 0 {
		return fmt.Errorf("empty shares provided")
	}

	// Verify the number of row shares matches the expected range from the proof
	if int64(len(shares)) != p.rowRootProof.End-p.rowRootProof.Start {
		return fmt.Errorf("incorrect number of row shares provided")
	}

	nth := nmt.NewNmtHasher(
		share.NewSHA256Hasher(),
		nmt_ns.ID(namespace.Bytes()).Size(),
		true,
	)

	// Initialize array to store computed row roots for each row of shares
	rowRoots := make([][]byte, len(shares))

	sharesProof := []*nmt.Proof{p.leftSharesProof, p.rightSharesProof}
	// Process rows that have proofs
	// These are typically incomplete rows (partial namespace data within a row)
	for _, proof := range sharesProof {
		if proof == nil {
			continue
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
		hashes, err := nmt.ComputePrefixedLeafHashes(nth, namespace.Bytes(), leaves)
		if err != nil {
			return fmt.Errorf("failed to compute leaf hashes: %w", err)
		}

		// Compute the row root using the namespace merkle tree proof
		root, err := proof.ComputeRootWithBasicValidation(nth, namespace.Bytes(), hashes, verifyNsCompleteness)
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
		LeftSharesProof  *nmt.Proof   `json:"left_shares_proof,omitempty"`
		RightSharesProof *nmt.Proof   `json:"right_shares_proof,omitempty"`
		RowRootProof     *MerkleProof `json:"row_root_proof,omitempty"`
	}{
		LeftSharesProof:  p.leftSharesProof,
		RightSharesProof: p.rightSharesProof,
		RowRootProof:     p.rowRootProof,
	}
	return json.Marshal(temp)
}

func (p *DataRootProof) UnmarshalJSON(data []byte) error {
	temp := struct {
		LeftSharesProof  *nmt.Proof   `json:"left_shares_proof,omitempty"`
		RightSharesProof *nmt.Proof   `json:"right_shares_proof,omitempty"`
		RowRootProof     *MerkleProof `json:"row_root_proof,omitempty"`
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	p.leftSharesProof = temp.LeftSharesProof
	p.rightSharesProof = temp.RightSharesProof
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

func reconstructEDS(shares [][]libshare.Share) (*share.AxisRoots, error) {
	rawShares := make([][]byte, 0, len(shares)*len(shares))
	for _, shares := range shares {
		rawShares = append(rawShares, libshare.ToBytes(shares)...)
	}

	treeFn := wrapper.NewConstructor(uint64(len(shares)))
	eds, err := rsmt2d.ComputeExtendedDataSquare(rawShares, share.DefaultRSMT2DCodec(), treeFn)
	if err != nil {
		return nil, err
	}
	return share.NewAxisRoots(eds)
}

func nmtToNmtPbProof(proof *nmt.Proof) *nmt_pb.Proof {
	if proof == nil {
		return nil
	}

	return &nmt_pb.Proof{
		Start:                 int64(proof.Start()),
		End:                   int64(proof.End()),
		Nodes:                 proof.Nodes(),
		LeafHash:              proof.LeafHash(),
		IsMaxNamespaceIgnored: proof.IsMaxNamespaceIDIgnored(),
	}
}

func pbNmtTonmtProof(pbproof *nmt_pb.Proof) *nmt.Proof {
	if pbproof == nil {
		return nil
	}
	proof := nmt.ProtoToProof(*pbproof)
	return &proof
}
