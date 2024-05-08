package shwap

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	types_pb "github.com/celestiaorg/celestia-node/share/shwap/proto"
)

// Sample represents a data share along with its Merkle proof, used to validate the share's
// inclusion in a data square.
type Sample struct {
	share.Share             // Embeds the share which includes the data with namespace.
	Proof       *nmt.Proof  // Proof is the Merkle Proof validating the share's inclusion.
	ProofType   rsmt2d.Axis // ProofType indicates whether the proof is against a row or a column.
}

// Validate checks the inclusion of the share using its Merkle proof under the specified root.
// Returns an error if the proof is invalid or does not correspond to the indicated proof type.
func (s *Sample) Validate(dah *share.Root, colIdx, rowIdx int) error {
	if s.ProofType != rsmt2d.Row && s.ProofType != rsmt2d.Col {
		return fmt.Errorf("invalid SampleProofType: %d", s.ProofType)
	}
	if !s.VerifyInclusion(dah, colIdx, rowIdx) {
		return fmt.Errorf("share proof is invalid")
	}
	return nil
}

// VerifyInclusion checks if the share is included in the given root hash at the specified indices.
func (s *Sample) VerifyInclusion(dah *share.Root, colIdx, rowIdx int) bool {
	rootHash := share.RootHashForCoordinates(dah, s.ProofType, uint(colIdx), uint(rowIdx))
	size := len(dah.RowRoots)
	isParity := colIdx >= size/2 || rowIdx >= size/2
	namespace := share.ParitySharesNamespace
	if !isParity {
		namespace = share.GetNamespace(s.Share)
	}

	return s.Proof.VerifyInclusion(
		sha256.New(), // Utilizes sha256, should be consistent throughout the application.
		namespace.ToNMT(),
		[][]byte{s.Share},
		rootHash,
	)
}

// ToProto converts a Sample into its protobuf representation for serialization purposes.
func (s *Sample) ToProto() *types_pb.Sample {
	return &types_pb.Sample{
		Share: &types_pb.Share{Data: s.Share},
		Proof: &nmt_pb.Proof{
			Start:                 int64(s.Proof.Start()),
			End:                   int64(s.Proof.End()),
			Nodes:                 s.Proof.Nodes(),
			LeafHash:              s.Proof.LeafHash(),
			IsMaxNamespaceIgnored: s.Proof.IsMaxNamespaceIDIgnored(),
		},
		ProofType: types_pb.AxisType(s.ProofType),
	}
}

// SampleFromEDS samples a share from an Extended Data Square based on the provided index and axis.
// This function generates a Merkle tree proof for the specified share.
func SampleFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	proofType rsmt2d.Axis,
	axisIdx, shrIdx int,
) (*Sample, error) {
	var shrs [][]byte
	switch proofType {
	case rsmt2d.Row:
		shrs = square.Row(uint(axisIdx))
	case rsmt2d.Col:
		shrs = square.Col(uint(axisIdx))
	default:
		return nil, fmt.Errorf("invalid proof type: %d", proofType)
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(square.Width()/2), uint(axisIdx))
	for _, shr := range shrs {
		err := tree.Push(shr)
		if err != nil {
			return nil, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	prf, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, fmt.Errorf("while proving range share over NMT: %w", err)
	}

	return &Sample{
		Share:     shrs[shrIdx],
		Proof:     &prf,
		ProofType: proofType,
	}, nil
}

// SampleFromProto converts a protobuf Sample back into its domain model equivalent.
func SampleFromProto(s *types_pb.Sample) *Sample {
	proof := nmt.NewInclusionProof(
		int(s.GetProof().GetStart()),
		int(s.GetProof().GetEnd()),
		s.GetProof().GetNodes(),
		s.GetProof().GetIsMaxNamespaceIgnored(),
	)
	return &Sample{
		Share:     ShareFromProto(s.GetShare()),
		Proof:     &proof,
		ProofType: rsmt2d.Axis(s.GetProofType()),
	}
}
