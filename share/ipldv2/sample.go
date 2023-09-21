package ipldv2

import (
	"crypto/sha256"
	"errors"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmtpb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
)

// SampleType represents type of sample.
type SampleType uint8

const (
	// DataSample is a sample of a data share.
	DataSample SampleType = iota
	// ParitySample is a sample of a parity share.
	ParitySample
)

// Sample represents a sample of an NMT in EDS.
type Sample struct {
	// ID of the Sample
	ID SampleID
	// Type of the Sample
	Type SampleType
	// Proof of Share inclusion in the NMT
	Proof nmt.Proof
	// Share being sampled
	Share share.Share
}

// NewSample constructs a new Sample.
func NewSample(root *share.Root, idx int, axis rsmt2d.Axis, shr share.Share, proof nmt.Proof) *Sample {
	id := NewSampleID(root, idx, axis)

	sqrLn := len(root.RowRoots)
	row, col := idx/sqrLn, idx%sqrLn
	tp := ParitySample
	if row < sqrLn/2 && col < sqrLn/2 {
		tp = DataSample
	}

	return &Sample{
		ID:    id,
		Type:  tp,
		Proof: proof,
		Share: shr,
	}
}

// NewSampleFrom samples the EDS and constructs a new Sample.
func NewSampleFrom(eds *rsmt2d.ExtendedDataSquare, idx int, axis rsmt2d.Axis) (*Sample, error) {
	sqrLn := int(eds.Width())
	axisIdx, shrIdx := idx/sqrLn, idx%sqrLn

	// TODO(@Wondertan): Should be an rsmt2d method
	var shrs [][]byte
	switch axis {
	case rsmt2d.Row:
		shrs = eds.Row(uint(axisIdx))
	case rsmt2d.Col:
		axisIdx, shrIdx = shrIdx, axisIdx
		shrs = eds.Col(uint(axisIdx))
	default:
		panic("invalid axis")
	}

	root, err := share.NewRoot(eds)
	if err != nil {
		return nil, err
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axisIdx))
	for _, shr := range shrs {
		err := tree.Push(shr)
		if err != nil {
			return nil, err
		}
	}

	proof, err := tree.ProveRange(shrIdx, shrIdx+1)
	if err != nil {
		return nil, err
	}

	return NewSample(root, idx, axis, shrs[shrIdx], proof), nil
}

// Proto converts Sample to its protobuf representation.
func (s *Sample) Proto() *ipldv2pb.Sample {
	// TODO: Extract as helper to nmt
	proof := &nmtpb.Proof{}
	proof.Nodes = s.Proof.Nodes()
	proof.End = int64(s.Proof.End())
	proof.Start = int64(s.Proof.Start())
	proof.IsMaxNamespaceIgnored = s.Proof.IsMaxNamespaceIDIgnored()
	proof.LeafHash = s.Proof.LeafHash()

	return &ipldv2pb.Sample{
		Id:    s.ID.Proto(),
		Type:  ipldv2pb.SampleType(s.Type),
		Proof: proof,
		Share: s.Share,
	}
}

// IPLDBlock converts Sample to an IPLD block for Bitswap compatibility.
func (s *Sample) IPLDBlock() (blocks.Block, error) {
	cid, err := s.ID.Cid()
	if err != nil {
		return nil, err
	}

	data, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, cid)
}

// MarshalBinary marshals Sample to binary.
func (s *Sample) MarshalBinary() ([]byte, error) {
	return s.Proto().Marshal()
}

// UnmarshalBinary unmarshals Sample from binary.
func (s *Sample) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.Sample{}
	err := proto.Unmarshal(data)
	if err != nil {
		return err
	}

	s.ID = SampleID{
		DataRoot: proto.Id.DataRoot,
		DAHRoot:  proto.Id.DahRoot,
		Index:    int(proto.Id.Index),
		Axis:     rsmt2d.Axis(proto.Id.Axis),
	}
	s.Type = SampleType(proto.Type)
	s.Proof = nmt.ProtoToProof(*proto.Proof)
	s.Share = proto.Share
	return nil
}

// Validate validates Sample's fields and proof of Share inclusion in the NMT.
func (s *Sample) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if s.Type != DataSample && s.Type != ParitySample {
		return errors.New("malformed sample type")
	}

	// TODO Support Col proofs
	namespace := share.ParitySharesNamespace
	if s.Type == DataSample {
		namespace = share.GetNamespace(s.Share)
	}

	if !s.Proof.VerifyInclusion(sha256.New(), namespace.ToNMT(), [][]byte{s.Share}, s.ID.DAHRoot) {
		return errors.New("sample proof is invalid")
	}

	return nil
}
