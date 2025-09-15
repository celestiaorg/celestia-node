package shwap

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-app/v6/pkg/wrapper"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// sampleName is the name identifier for the sample container.
const sampleName = "sample_v0"

// ErrFailedVerification is returned when inclusion proof verification fails. It is returned
// when the data and the proof do not match trusted data root.
var ErrFailedVerification = errors.New("failed to verify inclusion")

// Sample represents a data share along with its Merkle proof, used to validate the share's
// inclusion in a data square.
type Sample struct {
	libshare.Share             // Embeds the Share which includes the data with namespace.
	Proof          *nmt.Proof  // Proof is the Merkle Proof validating the share's inclusion.
	ProofType      rsmt2d.Axis // ProofType indicates whether the proof is against a row or a column.
}

// SampleFromShares creates a Sample from a list of shares, using the specified proof type and
// the share index to be included in the sample.
func SampleFromShares(shares []libshare.Share, proofType rsmt2d.Axis, idx SampleCoords) (Sample, error) {
	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(shares)/2), uint(idx.Row))
	for _, shr := range shares {
		err := tree.Push(shr.ToBytes())
		if err != nil {
			return Sample{}, err
		}
	}

	proof, err := tree.ProveRange(idx.Col, idx.Col+1)
	if err != nil {
		return Sample{}, err
	}

	return Sample{
		Share:     shares[idx.Col],
		Proof:     &proof,
		ProofType: proofType,
	}, nil
}

// SampleFromProto converts a protobuf Sample back into its domain model equivalent.
func SampleFromProto(s *pb.Sample) (Sample, error) {
	proof := nmt.NewInclusionProof(
		int(s.GetProof().GetStart()),
		int(s.GetProof().GetEnd()),
		s.GetProof().GetNodes(),
		s.GetProof().GetIsMaxNamespaceIgnored(),
	)

	shrs, err := ShareFromProto(s.GetShare())
	if err != nil {
		return Sample{}, err
	}

	return Sample{
		Share:     shrs,
		Proof:     &proof,
		ProofType: rsmt2d.Axis(s.GetProofType()),
	}, nil
}

// ToProto converts a Sample into its protobuf representation for serialization purposes.
func (s Sample) ToProto() *pb.Sample {
	return &pb.Sample{
		Share: &pb.Share{Data: s.ToBytes()},
		Proof: &nmt_pb.Proof{
			Start:                 int64(s.Proof.Start()),
			End:                   int64(s.Proof.End()),
			Nodes:                 s.Proof.Nodes(),
			LeafHash:              s.Proof.LeafHash(),
			IsMaxNamespaceIgnored: s.Proof.IsMaxNamespaceIDIgnored(),
		},
		ProofType: pb.AxisType(s.ProofType),
	}
}

// MarshalJSON encodes sample to the json encoded bytes.
func (s Sample) MarshalJSON() ([]byte, error) {
	jsonSample := struct {
		Share     libshare.Share `json:"share"`
		Proof     *nmt.Proof     `json:"proof"`
		ProofType rsmt2d.Axis    `json:"proof_type"`
	}{
		Share:     s.Share,
		Proof:     s.Proof,
		ProofType: s.ProofType,
	}
	return json.Marshal(&jsonSample)
}

// UnmarshalJSON decodes bytes to the Sample.
func (s *Sample) UnmarshalJSON(data []byte) error {
	var jsonSample struct {
		Share     libshare.Share `json:"share"`
		Proof     *nmt.Proof     `json:"proof"`
		ProofType rsmt2d.Axis    `json:"proof_type"`
	}
	if err := json.Unmarshal(data, &jsonSample); err != nil {
		return err
	}

	s.Share = jsonSample.Share
	s.Proof = jsonSample.Proof
	s.ProofType = jsonSample.ProofType

	return nil
}

// IsEmpty reports whether the Sample is empty, i.e. doesn't contain a proof.
func (s Sample) IsEmpty() bool {
	return s.Proof == nil
}

// Verify checks the inclusion of the share using its Merkle proof under the specified AxisRoots.
// Returns an error if the proof is invalid or does not correspond to the indicated proof type.
func (s Sample) Verify(roots *share.AxisRoots, rowIdx, colIdx int) error {
	if s.Proof == nil || s.Proof.IsEmptyProof() {
		return errors.New("nil proof")
	}
	if s.ProofType != rsmt2d.Row && s.ProofType != rsmt2d.Col {
		return fmt.Errorf("invalid SampleProofType: %d", s.ProofType)
	}
	if !s.verifyInclusion(roots, rowIdx, colIdx) {
		return ErrFailedVerification
	}
	return nil
}

func (s *Sample) WriteTo(writer io.Writer) (int64, error) {
	pbsample := s.ToProto()
	n, err := serde.Write(writer, pbsample)
	if err != nil {
		return int64(n), fmt.Errorf("writing Sample: %w", err)
	}

	return int64(n), nil
}

func (s *Sample) ReadFrom(reader io.Reader) (int64, error) {
	var sample pb.Sample
	n, err := serde.Read(reader, &sample)
	if err != nil {
		return int64(n), fmt.Errorf("reading Sample: %w", err)
	}
	*s, err = SampleFromProto(&sample)
	if err != nil {
		return 0, fmt.Errorf("unmarshaling Sample: %w", err)
	}
	return int64(n), nil
}

// verifyInclusion checks if the share is included in the given root hash at the specified indices.
func (s Sample) verifyInclusion(roots *share.AxisRoots, rowIdx, colIdx int) bool {
	size := len(roots.RowRoots)
	namespace := inclusionNamespace(s.Share, rowIdx, colIdx, size)
	rootHash := share.RootHashForCoordinates(roots, s.ProofType, uint(rowIdx), uint(colIdx))
	return s.Proof.VerifyInclusion(
		share.NewSHA256Hasher(),
		namespace.Bytes(),
		[][]byte{s.ToBytes()},
		rootHash,
	)
}

// inclusionNamespace returns the namespace for the share based on its position in the share.
// Shares from extended part of the square are considered parity shares. It means that
// parity shares are located outside of first quadrant of the square. According to the nmt
// specification, the parity shares are prefixed with the namespace of the parity shares.
func inclusionNamespace(sh libshare.Share, rowIdx, colIdx, squareSize int) libshare.Namespace {
	isParity := colIdx >= squareSize/2 || rowIdx >= squareSize/2
	if isParity {
		return libshare.ParitySharesNamespace
	}
	return sh.Namespace()
}
