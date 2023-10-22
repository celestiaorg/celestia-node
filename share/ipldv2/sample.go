package ipldv2

import (
	"errors"
	"fmt"

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
func NewSample(id SampleID, shr share.Share, proof nmt.Proof, sqrLn int) *Sample {
	tp := ParitySample
	if int(id.AxisIndex) < sqrLn/2 && int(id.ShareIndex) < sqrLn/2 {
		tp = DataSample
	}

	return &Sample{
		ID:    id,
		Type:  tp,
		Proof: proof,
		Share: shr,
	}
}

// NewSampleFromEDS samples the EDS and constructs a new Sample.
func NewSampleFromEDS(
	axisType rsmt2d.Axis,
	idx int,
	eds *rsmt2d.ExtendedDataSquare,
	height uint64,
) (*Sample, error) {
	sqrLn := int(eds.Width())
	axisIdx, shrIdx := idx/sqrLn, idx%sqrLn

	// TODO(@Wondertan): Should be an rsmt2d method
	var shrs [][]byte
	switch axisType {
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
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(axisIdx))
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

	id := NewSampleID(axisType, idx, root, height)
	return NewSample(id, shrs[shrIdx], prf, len(root.RowRoots)), nil
}

// SampleFromBlock converts blocks.Block into Sample.
func SampleFromBlock(blk blocks.Block) (*Sample, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	s := &Sample{}
	err := s.UnmarshalBinary(blk.RawData())
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling Sample: %w", err)
	}

	return s, nil
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
	id, err := s.ID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	proof := &nmtpb.Proof{}
	proof.Nodes = s.Proof.Nodes()
	proof.End = int64(s.Proof.End())
	proof.Start = int64(s.Proof.Start())
	proof.IsMaxNamespaceIgnored = s.Proof.IsMaxNamespaceIDIgnored()
	proof.LeafHash = s.Proof.LeafHash()

	return (&ipldv2pb.Sample{
		SampleId:    id,
		SampleType:  ipldv2pb.SampleType(s.Type),
		SampleProof: proof,
		SampleShare: s.Share,
	}).Marshal()
}

// UnmarshalBinary unmarshal Sample from binary.
func (s *Sample) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.Sample{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	err := s.ID.UnmarshalBinary(proto.SampleId)
	if err != nil {
		return err
	}

	s.Type = SampleType(proto.SampleType)
	s.Proof = nmt.ProtoToProof(*proto.SampleProof)
	s.Share = proto.SampleShare
	return nil
}

// Validate validates Sample's fields and proof of Share inclusion in the NMT.
func (s *Sample) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if s.Type != DataSample && s.Type != ParitySample {
		return fmt.Errorf("incorrect sample type: %d", s.Type)
	}

	namespace := share.ParitySharesNamespace
	if s.Type == DataSample {
		namespace = share.GetNamespace(s.Share)
	}

	s.Proof.WithHashedProof(hasher())
	if !s.Proof.VerifyInclusion(hasher(), namespace.ToNMT(), [][]byte{s.Share}, s.ID.AxisHash) {
		return errors.New("sample proof is invalid")
	}

	return nil
}
