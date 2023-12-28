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

// SampleProofType is either row or column proven Sample.
type SampleProofType = rsmt2d.Axis

const (
	// RowProofType is a sample proven via row root of the square.
	RowProofType = rsmt2d.Row
	// ColProofType is a sample proven via column root of the square.
	ColProofType = rsmt2d.Col
)

// Sample represents a sample of an NMT in EDS.
type Sample struct {
	SampleID

	// SampleProofType of the Sample
	SampleProofType SampleProofType
	// SampleProof of SampleShare inclusion in the NMT
	SampleProof nmt.Proof
	// SampleShare is a share being sampled
	SampleShare share.Share
}

// NewSample constructs a new Sample.
func NewSample(id SampleID, shr share.Share, proof nmt.Proof, proofTp SampleProofType) *Sample {
	return &Sample{
		SampleID:        id,
		SampleProofType: proofTp,
		SampleProof:     proof,
		SampleShare:     shr,
	}
}

// NewSampleFromEDS samples the EDS and constructs a new row-proven Sample.
func NewSampleFromEDS(
	proofType SampleProofType,
	smplIdx int,
	square *rsmt2d.ExtendedDataSquare,
	height uint64,
) (*Sample, error) {
	root, err := share.NewRoot(square)
	if err != nil {
		return nil, err
	}

	id, err := NewSampleID(height, smplIdx, root)
	if err != nil {
		return nil, err
	}

	sqrLn := int(square.Width())
	rowIdx, shrIdx := uint16(smplIdx/sqrLn), uint16(smplIdx%sqrLn)

	// TODO(@Wondertan): Should be an rsmt2d method
	var shrs [][]byte
	switch proofType {
	case rsmt2d.Row:
		shrs = square.Row(uint(rowIdx))
	case rsmt2d.Col:
		rowIdx, shrIdx = shrIdx, rowIdx
		shrs = square.Col(uint(rowIdx))
	default:
		panic("invalid axis")
	}

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(sqrLn/2), uint(rowIdx))
	for _, shr := range shrs {
		err := tree.Push(shr)
		if err != nil {
			return nil, fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	prf, err := tree.ProveRange(int(shrIdx), int(shrIdx+1))
	if err != nil {
		return nil, fmt.Errorf("while proving range share over NMT: %w", err)
	}

	return NewSample(id, shrs[shrIdx], prf, proofType), nil
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
	data, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, s.Cid())
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
		SampleType:  shwappb.SampleProofType(s.SampleProofType),
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

	s.SampleProofType = SampleProofType(proto.SampleType)
	s.SampleProof = nmt.ProtoToProof(*proto.SampleProof)
	s.SampleShare = proto.SampleShare
	return nil
}

// Verify validates Sample's fields and verifies SampleShare inclusion.
func (s *Sample) Verify(root *share.Root) error {
	if err := s.SampleID.Verify(root); err != nil {
		return err
	}

	if s.SampleProofType != RowProofType && s.SampleProofType != ColProofType {
		return fmt.Errorf("invalid SampleProofType: %d", s.SampleProofType)
	}

	sqrLn := len(root.RowRoots)
	namespace := share.ParitySharesNamespace
	if int(s.RowIndex) < sqrLn/2 && int(s.ShareIndex) < sqrLn/2 {
		namespace = share.GetNamespace(s.SampleShare)
	}

	rootHash := root.RowRoots[s.RowIndex]
	if s.SampleProofType == ColProofType {
		rootHash = root.ColumnRoots[s.ShareIndex]
	}

	if !s.SampleProof.VerifyInclusion(hashFn(), namespace.ToNMT(), [][]byte{s.SampleShare}, rootHash) {
		return errors.New("invalid Sample")
	}

	return nil
}
