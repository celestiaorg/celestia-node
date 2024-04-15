package shwap

import (
	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// Sample represents a sample of an NMT in EDS.
type Sample struct {
	SampleID
	*share.ShareWithProof
}

// SampleFromBlock converts blocks.Block into Sample.
func SampleFromBlock(blk blocks.Block) (*Sample, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}
	sample := new(shwappb.SampleBlock)
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

// ToProto marshals Sample to proto.
func (s *Sample) ToProto() *shwappb.SampleBlock {
	return &shwappb.SampleBlock{
		SampleId: s.SampleID.MarshalBinary(),
		Sample:   s.ShareWithProof.ToProto(),
	}
}

// SampleFromProto unmarshal Sample from proto.
func SampleFromProto(sampleProto *shwappb.SampleBlock) (*Sample, error) {
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
