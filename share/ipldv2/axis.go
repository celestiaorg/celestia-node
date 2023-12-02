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

// Axis represents an Axis of an EDS.
type Axis struct {
	AxisID

	// AxisShares is the original half of the axis.
	AxisShares []share.Share
}

// NewAxis constructs a new Axis.
func NewAxis(id AxisID, axisHalf []share.Share) *Axis {
	return &Axis{
		AxisID:     id,
		AxisShares: axisHalf,
	}
}

// NewAxisFromEDS samples the EDS and constructs a new Axis.
func NewAxisFromEDS(
	axisType rsmt2d.Axis,
	axisIdx int,
	square *rsmt2d.ExtendedDataSquare,
	height uint64,
) (*Axis, error) {
	sqrLn := int(square.Width())

	// TODO(@Wondertan): Should be an rsmt2d method
	var axisHalf [][]byte
	switch axisType {
	case rsmt2d.Row:
		axisHalf = square.Row(uint(axisIdx))[:sqrLn/2]
	case rsmt2d.Col:
		axisHalf = square.Col(uint(axisIdx))[:sqrLn/2]
	default:
		panic("invalid axis")
	}

	root, err := share.NewRoot(square)
	if err != nil {
		return nil, fmt.Errorf("while computing root: %w", err)
	}

	id := NewAxisID(axisType, uint16(axisIdx), root, height)
	return NewAxis(id, axisHalf), nil
}

// AxisFromBlock converts blocks.Block into Axis.
func AxisFromBlock(blk blocks.Block) (*Axis, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	s := &Axis{}
	err := s.UnmarshalBinary(blk.RawData())
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling Axis: %w", err)
	}

	return s, nil
}

// IPLDBlock converts Axis to an IPLD block for Bitswap compatibility.
func (s *Axis) IPLDBlock() (blocks.Block, error) {
	cid, err := s.AxisID.Cid()
	if err != nil {
		return nil, err
	}

	data, err := s.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, cid)
}

// MarshalBinary marshals Axis to binary.
func (s *Axis) MarshalBinary() ([]byte, error) {
	id, err := s.AxisID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return (&ipldv2pb.Axis{
		AxisId:   id,
		AxisHalf: s.AxisShares,
	}).Marshal()
}

// UnmarshalBinary unmarshal Axis from binary.
func (s *Axis) UnmarshalBinary(data []byte) error {
	proto := &ipldv2pb.Axis{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	err := s.AxisID.UnmarshalBinary(proto.AxisId)
	if err != nil {
		return err
	}

	s.AxisShares = proto.AxisHalf
	return nil
}

// Validate validates Axis's fields and proof of axis inclusion.
func (s *Axis) Validate() error {
	if err := s.AxisID.Validate(); err != nil {
		return err
	}

	sqrLn := len(s.AxisShares) * 2
	if s.AxisID.AxisIndex > uint16(sqrLn) {
		return fmt.Errorf("axis index exceeds square size: %d > %d", s.AxisID.AxisIndex, sqrLn)
	}

	// TODO(@Wondertan): This computations are quite expensive and likely to be used further,
	//  so we need to find a way to cache them and pass to the caller on the Bitswap side
	parity, err := share.DefaultRSMT2DCodec().Encode(s.AxisShares)
	if err != nil {
		return fmt.Errorf("while decoding erasure coded half: %w", err)
	}
	s.AxisShares = append(s.AxisShares, parity...)

	tree := wrapper.NewErasuredNamespacedMerkleTree(uint64(len(s.AxisShares)/2), uint(s.AxisID.AxisIndex))
	for _, shr := range s.AxisShares {
		err := tree.Push(shr)
		if err != nil {
			return fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	root, err := tree.Root()
	if err != nil {
		return fmt.Errorf("while computing NMT root: %w", err)
	}

	hashedRoot := hashBytes(root)
	if !bytes.Equal(s.AxisID.AxisHash, hashedRoot) {
		return fmt.Errorf("invalid axis hash: %X != %X", root, s.AxisID.AxisHash)
	}

	return nil
}
