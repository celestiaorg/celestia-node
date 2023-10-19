package ipldv2

import (
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
)

// AxisSampleIDSize is the size of the AxisSampleID in bytes
const AxisSampleIDSize = 103

// AxisSampleID is an unique identifier of a AxisSample.
type AxisSampleID struct {
	// Height of the block.
	// Needed to identify block's data square in the whole chain
	Height uint64
	// AxisHash is the Col or AxisSample root from DAH of the data square
	AxisHash []byte
	// Index is the index of the axis(row, col) in the data square
	Index int
	// Axis is Col or AxisSample axis of the sample in the data square
	Axis rsmt2d.Axis
}

// NewAxisSampleID constructs a new AxisSampleID.
func NewAxisSampleID(height uint64, root *share.Root, idx int, axis rsmt2d.Axis) AxisSampleID {
	dahroot := root.RowRoots[idx]
	if axis == rsmt2d.Col {
		dahroot = root.ColumnRoots[idx]
	}

	return AxisSampleID{
		Height:   height,
		AxisHash: dahroot,
		Index:    idx,
		Axis:     axis,
	}
}

// AxisSampleIDFromCID coverts CID to AxisSampleID.
func AxisSampleIDFromCID(cid cid.Cid) (id AxisSampleID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhalling AxisSampleID: %w", err)
	}

	return id, nil
}

// AxisSampleIDFromProto converts from protobuf representation of AxisSampleID.
func AxisSampleIDFromProto(proto *ipldv2pb.AxisSampleID) AxisSampleID {
	return AxisSampleID{
		Height:   proto.Height,
		AxisHash: proto.AxisHash,
		Index:    int(proto.Index),
		Axis:     rsmt2d.Axis(proto.Axis),
	}
}

// Cid returns sample ID encoded as CID.
func (s *AxisSampleID) Cid() (cid.Cid, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, axisSamplingMultihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(axisSamplingCodec, buf), nil
}

// Proto converts AxisSampleID to its protobuf representation.
func (s *AxisSampleID) Proto() *ipldv2pb.AxisSampleID {
	return &ipldv2pb.AxisSampleID{
		Height:   s.Height,
		AxisHash: s.AxisHash,
		Index:    uint32(s.Index),
		Axis:     ipldv2pb.Axis(s.Axis),
	}
}

// MarshalBinary encodes AxisSampleID into binary form.
func (s *AxisSampleID) MarshalBinary() ([]byte, error) {
	// we cannot use protobuf here because it exceeds multihash limit of 128 bytes
	data := make([]byte, 0, AxisSampleIDSize)
	data = binary.LittleEndian.AppendUint64(data, s.Height)
	data = append(data, s.AxisHash...)
	data = binary.LittleEndian.AppendUint32(data, uint32(s.Index))
	data = append(data, byte(s.Axis))
	return data, nil
}

// UnmarshalBinary decodes AxisSampleID from binary form.
func (s *AxisSampleID) UnmarshalBinary(data []byte) error {
	if len(data) != AxisSampleIDSize {
		return fmt.Errorf("incorrect sample id size: %d != %d", len(data), AxisSampleIDSize)
	}

	s.Height = binary.LittleEndian.Uint64(data)
	s.AxisHash = append(s.AxisHash, data[8:8+dahRootSize]...) // copying data to avoid slice aliasing
	s.Index = int(binary.LittleEndian.Uint32(data[8+dahRootSize : 8+dahRootSize+4]))
	s.Axis = rsmt2d.Axis(data[8+dahRootSize+4])
	return nil
}

// Validate validates fields of AxisSampleID.
func (s *AxisSampleID) Validate() error {
	if s.Height == 0 {
		return fmt.Errorf("zero Height")
	}

	if len(s.AxisHash) != dahRootSize {
		return fmt.Errorf("incorrect AxisHash size: %d != %d", len(s.AxisHash), dahRootSize)
	}

	if s.Axis != rsmt2d.Col && s.Axis != rsmt2d.Row {
		return fmt.Errorf("incorrect Axis: %d", s.Axis)
	}

	return nil
}
