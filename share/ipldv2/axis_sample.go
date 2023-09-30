package ipldv2

import (
	"bytes"
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
)

type AxisSample struct {
	ID       AxisSampleID
	AxisHalf []share.Share
}

// NewAxisSample constructs a new AxisSample.
func NewAxisSample(id AxisSampleID, axisHalf []share.Share) *AxisSample {
	return &AxisSample{
		ID:       id,
		AxisHalf: axisHalf,
	}
}

// NewAxisSampleFrom constructs a new AxisSample from share.Root.
func NewAxisSampleFrom(root *share.Root, idx int, axis rsmt2d.Axis, axisHalf []share.Share) *AxisSample {
	id := NewAxisSampleID(root, idx, axis)
	return NewAxisSample(id, axisHalf)
}

// NewAxisSampleFromEDS samples the EDS and constructs a new AxisSample.
func NewAxisSampleFromEDS(eds *rsmt2d.ExtendedDataSquare, idx int, axis rsmt2d.Axis) (*AxisSample, error) {
	sqrLn := int(eds.Width())

	// TODO(@Wondertan): Should be an rsmt2d method
	var axisHalf [][]byte
	switch axis {
	case rsmt2d.Row:
		axisHalf = eds.Row(uint(idx))[:sqrLn/2]
	case rsmt2d.Col:
		axisHalf = eds.Col(uint(idx))[:sqrLn/2]
	default:
		panic("invalid axis")
	}

	root, err := share.NewRoot(eds)
	if err != nil {
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	return NewAxisSampleFrom(root, idx, axis, axisHalf), nil
}

// Proto converts AxisSample to its protobuf representation.
func (s *AxisSample) Proto() *ipldv2pb.AxisSample {
	return &ipldv2pb.AxisSample{
		Id:       s.ID.Proto(),
		AxisHalf: s.AxisHalf,
	}
}

// AxisSampleFromBlock converts blocks.Block into AxisSample.
func AxisSampleFromBlock(blk blocks.Block) (*AxisSample, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	s := &AxisSample{}
	err := s.UnmarshalBinary(blk.RawData())
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling ShareSample: %w", err)
	}

	return s, nil
}

// IPLDBlock converts AxisSample to an IPLD block for Bitswap compatibility.
func (s *AxisSample) IPLDBlock() (blocks.Block, error) {
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

// MarshalBinary marshals AxisSample to binary.
func (s *AxisSample) MarshalBinary() ([]byte, error) {
	return s.Proto().Marshal()
}

// UnmarshalBinary unmarshal AxisSample from binary.
func (s *AxisSample) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.AxisSample{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	s.ID = AxisSampleID{
		DataHash: proto.Id.DataHash,
		AxisHash: proto.Id.AxisHash,
		Index:    int(proto.Id.Index),
		Axis:     rsmt2d.Axis(proto.Id.Axis),
	}
	s.AxisHalf = proto.AxisHalf
	return nil
}

// Validate validates AxisSample's fields and proof of Share inclusion in the NMT.
func (s *AxisSample) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	sqrLn := len(s.AxisHalf) * 2
	if s.ID.Index > sqrLn {
		return fmt.Errorf("row index exceeds square size: %d > %d", s.ID.Index, sqrLn)
	}

	// TODO(@Wondertan): This computations are quite expensive and likely to be used further,
	//  so we need to find a way to cache them and pass to the caller on the Bitswap side
	parity, err := share.DefaultRSMT2DCodec().Encode(s.AxisHalf)
	if err != nil {
		return fmt.Errorf("while decoding erasure coded half: %w", err)
	}
	s.AxisHalf = append(s.AxisHalf, parity...)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(s.AxisHalf)), uint(s.ID.Index))
	for _, shr := range s.AxisHalf {
		err := tree.Push(shr)
		if err != nil {
			return fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return fmt.Errorf("while computing NMT root: %w", err)
	}

	if !bytes.Equal(s.ID.AxisHash, root) {
		return fmt.Errorf("invalid root: %X != %X", root, s.ID.AxisHash)
	}

	return nil
}
