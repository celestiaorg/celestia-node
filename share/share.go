package share

import (
	"crypto/sha256"
	"fmt"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	types_pb "github.com/celestiaorg/celestia-node/share/pb"
)

var (
	// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
	DefaultRSMT2DCodec = appconsts.DefaultCodec
)

const (
	// Size is a system-wide size of a share, including both data and namespace GetNamespace
	Size = appconsts.ShareSize
)

var (
	// MaxSquareSize is currently the maximum size supported for unerasured data in
	// rsmt2d.ExtendedDataSquare.
	MaxSquareSize = appconsts.SquareSizeUpperBound(appconsts.LatestVersion)
)

// Share contains the raw share data without the corresponding namespace.
// NOTE: Alias for the byte is chosen to keep maximal compatibility, especially with rsmt2d.
// Ideally, we should define reusable type elsewhere and make everyone(Core, rsmt2d, ipld) to rely
// on it.
type Share = []byte

// GetNamespace slices Namespace out of the Share.
func GetNamespace(s Share) Namespace {
	return s[:NamespaceSize]
}

// GetData slices out data of the Share.
func GetData(s Share) []byte {
	return s[NamespaceSize:]
}

// ShareWithProof contains data with corresponding Merkle Proof
type ShareWithProof struct { //nolint: revive
	// Share is a full data including namespace
	Share
	// Proof is a Merkle Proof of current share
	Proof *nmt.Proof
	// ProofType is a type of axis against which the share proof is computed
	ProofType rsmt2d.Axis
}

func (s *ShareWithProof) Validate(dah *Root, x, y int) error {
	if s.ProofType != rsmt2d.Row && s.ProofType != rsmt2d.Col {
		return fmt.Errorf("invalid SampleProofType: %d", s.ProofType)
	}
	if !s.VerifyInclusion(dah, x, y) {
		return fmt.Errorf("share proof is invalid")
	}
	return nil
}

// Validate validates inclusion of the share under the given root CID.
func (s *ShareWithProof) VerifyInclusion(dah *Root, x, y int) bool {
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

func (s *ShareWithProof) ToProto() *types_pb.ShareWithProof {
	return &types_pb.ShareWithProof{
		Share: s.Share,
		Proof: &nmt_pb.Proof{
			Start:                 int64(s.Proof.Start()),
			End:                   int64(s.Proof.End()),
			Nodes:                 s.Proof.Nodes(),
			LeafHash:              s.Proof.LeafHash(),
			IsMaxNamespaceIgnored: s.Proof.IsMaxNamespaceIDIgnored(),
		},
		ProofType: types_pb.Axis(s.ProofType),
	}
}

// newSampleFromEDS samples the EDS and constructs a new row-proven Sample.
func ShareWithProofFromEDS(
	square *rsmt2d.ExtendedDataSquare,
	proofType rsmt2d.Axis,
	axisIdx, shrIdx int,
) (*ShareWithProof, error) {
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

	sp := &ShareWithProof{
		Share:     shrs[shrIdx],
		Proof:     &prf,
		ProofType: proofType,
	}
	return sp, nil
}

func ShareWithProofFromProto(s *types_pb.ShareWithProof) *ShareWithProof {
	proof := nmt.NewInclusionProof(
		int(s.Proof.Start),
		int(s.Proof.End),
		s.Proof.Nodes,
		s.Proof.IsMaxNamespaceIgnored,
	)
	return &ShareWithProof{
		Share:     s.Share,
		Proof:     &proof,
		ProofType: rsmt2d.Axis(s.ProofType),
	}
}
