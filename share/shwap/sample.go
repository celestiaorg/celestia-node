package shwap

import (
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// Sample represents a sample of an NMT in EDS.
type Sample struct {
	SampleID
	*share.ShareWithProof
}

// NewSample constructs a new Sample.
func NewSample(id SampleID, s *share.ShareWithProof) *Sample {
	return &Sample{
		SampleID:       id,
		ShareWithProof: s,
	}
}

// NewSampleFromEDS samples the EDS and constructs a new row-proven Sample.
func NewSampleFromEDS(
	proofType rsmt2d.Axis,
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

	sp := &share.ShareWithProof{
		Share: shrs[shrIdx],
		Proof: &prf,
		Axis:  proofType,
	}
	return NewSample(id, sp), nil
}

// SampleFromBlock converts blocks.Block into Sample.
func SampleFromBlock(blk blocks.Block) (*Sample, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}
	sample := new(shwappb.SampleResponse)
	if err := sample.Unmarshal(blk.RawData()); err != nil {
		return nil, err
	}
	return SampleFromProto(sample)
}

// IPLDBlock converts Sample to an IPLD block for Bitswap compatibility.
func (s *Sample) IPLDBlock() (blocks.Block, error) {
	data, err := s.ToProto().Marshal()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, s.Cid())
}

// MarshalBinary marshals Sample to binary.
func (s *Sample) ToProto() *shwappb.SampleResponse {
	return &shwappb.SampleResponse{
		SampleId: s.SampleID.MarshalBinary(),
		Sample:   s.ShareWithProof.ToProto(),
	}
}

// SampleFromBinary unmarshal Sample from binary.
func SampleFromProto(sampleProto *shwappb.SampleResponse) (*Sample, error) {
	id, err := SampleIdFromBinary(sampleProto.SampleId)
	if err != nil {
		return nil, err
	}
	return &Sample{
		SampleID:       id,
		ShareWithProof: share.ShareWithProofFromProto(sampleProto.Sample),
	}, nil
}

// Verify validates Sample's fields and verifies SampleShare inclusion.
func (s *Sample) Verify(root *share.Root) error {
	if err := s.SampleID.Verify(root); err != nil {
		return err
	}

	x, y := int(s.RowIndex), int(s.ShareIndex)
	return s.Validate(root, x, y)
}
