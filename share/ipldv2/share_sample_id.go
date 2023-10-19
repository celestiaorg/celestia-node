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

// ShareSampleIDSize is the size of the ShareSampleID in bytes
const ShareSampleIDSize = 45

// ShareSampleID is an unique identifier of a ShareSample.
type ShareSampleID struct {
	// Height of the block.
	// Needed to identify block's data square in the whole chain
	Height uint64
	// AxisHash is the sha256 hash of a Col or Row root taken from DAH of the data square
	AxisHash []byte
	// Index is the index of the sampled share in the data square(not row or col index)
	Index int
	// Axis is Col or Row axis of the sample in the data square
	Axis rsmt2d.Axis
}

// NewShareSampleID constructs a new ShareSampleID.
func NewShareSampleID(height uint64, root *share.Root, idx int, axis rsmt2d.Axis) ShareSampleID {
	sqrLn := len(root.RowRoots)
	row, col := idx/sqrLn, idx%sqrLn
	dahroot := root.RowRoots[row]
	if axis == rsmt2d.Col {
		dahroot = root.ColumnRoots[col]
	}
	axisHash := hashBytes(dahroot)

	return ShareSampleID{
		Height:   height,
		AxisHash: axisHash,
		Index:    idx,
		Axis:     axis,
	}
}

// ShareSampleIDFromCID coverts CID to ShareSampleID.
func ShareSampleIDFromCID(cid cid.Cid) (id ShareSampleID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhalling ShareSampleID: %w", err)
	}

	return id, nil
}

// ShareSampleIDFromProto converts from protobuf representation of ShareSampleID.
func ShareSampleIDFromProto(proto *ipldv2pb.ShareSampleID) ShareSampleID {
	return ShareSampleID{
		Height:   proto.Height,
		AxisHash: proto.AxisHash,
		Index:    int(proto.Index),
		Axis:     rsmt2d.Axis(proto.Axis),
	}
}

// Cid returns sample ID encoded as CID.
func (s *ShareSampleID) Cid() (cid.Cid, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, shareSamplingMultihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(shareSamplingCodec, buf), nil
}

// Proto converts ShareSampleID to its protobuf representation.
func (s *ShareSampleID) Proto() *ipldv2pb.ShareSampleID {
	return &ipldv2pb.ShareSampleID{
		Height:   s.Height,
		AxisHash: s.AxisHash,
		Index:    uint32(s.Index),
		Axis:     ipldv2pb.Axis(s.Axis),
	}
}

// MarshalBinary encodes ShareSampleID into binary form.
func (s *ShareSampleID) MarshalBinary() ([]byte, error) {
	// we cannot use protobuf here because it exceeds multihash limit of 128 bytes
	data := make([]byte, 0, ShareSampleIDSize)
	data = binary.LittleEndian.AppendUint64(data, s.Height)
	data = append(data, s.AxisHash...)
	data = binary.LittleEndian.AppendUint32(data, uint32(s.Index))
	data = append(data, byte(s.Axis))
	return data, nil
}

// UnmarshalBinary decodes ShareSampleID from binary form.
func (s *ShareSampleID) UnmarshalBinary(data []byte) error {
	if len(data) != ShareSampleIDSize {
		return fmt.Errorf("incorrect SampleID size: %d != %d", len(data), ShareSampleIDSize)
	}

	s.Height = binary.LittleEndian.Uint64(data)
	s.AxisHash = append(s.AxisHash, data[8:8+hashSize]...) // copying data to avoid slice aliasing
	s.Index = int(binary.LittleEndian.Uint32(data[8+hashSize : 8+hashSize+4]))
	s.Axis = rsmt2d.Axis(data[8+hashSize+4])
	return nil
}

// Validate validates fields of ShareSampleID.
func (s *ShareSampleID) Validate() error {
	if s.Height == 0 {
		return fmt.Errorf("zero Height")
	}

	if len(s.AxisHash) != hashSize {
		return fmt.Errorf("incorrect AxisHash size: %d != %d", len(s.AxisHash), hashSize)
	}

	if s.Axis != rsmt2d.Col && s.Axis != rsmt2d.Row {
		return fmt.Errorf("incorrect Axis: %d", s.Axis)
	}

	return nil
}
