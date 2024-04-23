package share

import (
	"crypto/sha256"
	"fmt"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	types_pb "github.com/celestiaorg/celestia-node/share/pb"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"
)

// Sample contains share with corresponding Merkle Proof
type Sample struct { //nolint: revive
	// Share is a full data including namespace
	Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// ProofType is a type of axis against which the share proof is computed
	ProofType rsmt2d.Axis
}

func (s *Sample) Validate(dah *Root, x, y int) error {
	if s.ProofType != rsmt2d.Row && s.ProofType != rsmt2d.Col {
		return fmt.Errorf("invalid SampleProofType: %d", s.ProofType)
	}
	if !s.VerifyInclusion(dah, x, y) {
		return fmt.Errorf("share proof is invalid")
	}
	return nil
}

// Validate validates inclusion of the share under the given root CID.
func (s *Sample) VerifyInclusion(dah *Root, x, y int) bool {
	rootHash := rootHashForCoordinates(dah, s.ProofType, uint(x), uint(y))

	size := len(dah.RowRoots)
	isParity := x >= size/2 || y >= size/2
	namespace := ParitySharesNamespace
	if !isParity {
		namespace = GetNamespace(s.Share)
	}

	return s.Proof.VerifyInclusion(
		sha256.New(), // TODO(@Wondertan): This should be defined somewhere globally
		namespace.ToNMT(),
		[][]byte{s.Share},
		rootHash,
	)
}

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

// SampleFromEDS samples the EDS and constructs a new row-proven Sample.
func SampleFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	proofType rsmt2d.Axis,
	axisIdx, shrIdx int,
) (*Sample, error) {
	// TODO(@Wondertan): Should be an rsmt2d method
	var shrs [][]byte
	switch proofType {
	case rsmt2d.Row:
		shrs = square.Row(uint(axisIdx))
	case rsmt2d.Col:
		shrs = square.Col(uint(axisIdx))
	default:
		panic("invalid axis")
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

	sp := &Sample{
		Share:     shrs[shrIdx],
		Proof:     &prf,
		ProofType: proofType,
	}
	return sp, nil
}

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
