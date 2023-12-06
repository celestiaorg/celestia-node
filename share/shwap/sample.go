package shwap

import (
	"errors"
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	nmtpb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
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
	SampleID

	// Type of the Sample
	Type SampleType
	// SampleProof of SampleShare inclusion in the NMT
	SampleProof nmt.Proof
	// SampleShare is a share being sampled
	SampleShare share.Share
}

// NewSample constructs a new Sample.
func NewSample(id SampleID, shr share.Share, proof nmt.Proof, sqrLn int) *Sample {
	tp := ParitySample
	if int(id.AxisIndex) < sqrLn/2 && int(id.ShareIndex) < sqrLn/2 {
		tp = DataSample
	}

	return &Sample{
		SampleID:    id,
		Type:        tp,
		SampleProof: proof,
		SampleShare: shr,
	}
}

// NewSampleFromEDS samples the EDS and constructs a new Sample.
func NewSampleFromEDS(
	axisType rsmt2d.Axis,
	idx int,
	square *rsmt2d.ExtendedDataSquare,
	height uint64,
) (*Sample, error) {
	sqrLn := int(square.Width())
	axisIdx, shrIdx := idx/sqrLn, idx%sqrLn

	// TODO(@Wondertan): Should be an rsmt2d method
	var shrs [][]byte
	switch axisType {
	case rsmt2d.Row:
		shrs = square.Row(uint(axisIdx))
	case rsmt2d.Col:
		axisIdx, shrIdx = shrIdx, axisIdx
		shrs = square.Col(uint(axisIdx))
	default:
		panic("invalid axis")
	}

	root, err := share.NewRoot(square)
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
	cid, err := s.SampleID.Cid()
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
	id, err := s.SampleID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	proof := &nmtpb.Proof{}
	proof.Nodes = s.SampleProof.Nodes()
	proof.End = int64(s.SampleProof.End())
	proof.Start = int64(s.SampleProof.Start())
	proof.IsMaxNamespaceIgnored = s.SampleProof.IsMaxNamespaceIDIgnored()
	proof.LeafHash = s.SampleProof.LeafHash()

	return (&shwappb.Sample{
		SampleId:    id,
		SampleType:  shwappb.SampleType(s.Type),
		SampleProof: proof,
		SampleShare: s.SampleShare,
	}).Marshal()
}

// UnmarshalBinary unmarshal Sample from binary.
func (s *Sample) UnmarshalBinary(data []byte) error {
	proto := &shwappb.Sample{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	err := s.SampleID.UnmarshalBinary(proto.SampleId)
	if err != nil {
		return err
	}

	s.Type = SampleType(proto.SampleType)
	s.SampleProof = nmt.ProtoToProof(*proto.SampleProof)
	s.SampleShare = proto.SampleShare
	return nil
}

// Validate validates Sample's fields and proof of SampleShare inclusion in the NMT.
func (s *Sample) Validate() error {
	if err := s.SampleID.Validate(); err != nil {
		return err
	}

	if s.Type != DataSample && s.Type != ParitySample {
		return fmt.Errorf("invalid SampleType: %d", s.Type)
	}

	namespace := share.ParitySharesNamespace
	if s.Type == DataSample {
		namespace = share.GetNamespace(s.SampleShare)
	}

	s.SampleProof.WithHashedProof(hasher())
	if !s.SampleProof.VerifyInclusion(hasher(), namespace.ToNMT(), [][]byte{s.SampleShare}, s.AxisHash) {
		return errors.New("invalid ")
	}

	return nil
}
