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
const AxisSampleIDSize = 127

// AxisSampleID is an unique identifier of a AxisSample.
type AxisSampleID struct {
	// DataHash is the root of the data square
	// Needed to identify the data square in the whole chain
	DataHash share.DataHash
	// AxisHash is the Col or AxisSample root from DAH of the data square
	AxisHash []byte
	// Index is the index of the sample in the data square(not row or col index)
	Index int
	// Axis is Col or AxisSample axis of the sample in the data square
	Axis rsmt2d.Axis
}

// NewAxisSampleID constructs a new AxisSampleID.
func NewAxisSampleID(root *share.Root, idx int, axis rsmt2d.Axis) AxisSampleID {
	dahroot := root.RowRoots[idx]
	if axis == rsmt2d.Col {
		dahroot = root.ColumnRoots[idx]
	}

	return AxisSampleID{
		DataHash: root.Hash(),
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
		DataHash: s.DataHash,
		AxisHash: s.AxisHash,
		Index:    uint32(s.Index),
		Axis:     ipldv2pb.Axis(s.Axis),
	}
}

// MarshalBinary encodes AxisSampleID into binary form.
func (s *AxisSampleID) MarshalBinary() ([]byte, error) {
	// we cannot use protobuf here because it exceeds multihash limit of 128 bytes
	data := make([]byte, ShareSampleIDSize)
	n := copy(data, s.DataHash)
	n += copy(data[n:], s.AxisHash)
	binary.LittleEndian.PutUint32(data[n:], uint32(s.Index))
	data[n+4] = byte(s.Axis)
	return data, nil
}

// UnmarshalBinary decodes AxisSampleID from binary form.
func (s *AxisSampleID) UnmarshalBinary(data []byte) error {
	if len(data) != ShareSampleIDSize {
		return fmt.Errorf("incorrect sample id size: %d != %d", len(data), ShareSampleIDSize)
	}

	// copying data to avoid slice aliasing
	s.DataHash = append(s.DataHash, data[:hashSize]...)
	s.AxisHash = append(s.AxisHash, data[hashSize:hashSize+dahRootSize]...)
	s.Index = int(binary.LittleEndian.Uint32(data[hashSize+dahRootSize : hashSize+dahRootSize+4]))
	s.Axis = rsmt2d.Axis(data[hashSize+dahRootSize+4])
	return nil
}

// Validate validates fields of AxisSampleID.
func (s *AxisSampleID) Validate() error {
	if len(s.DataHash) != hashSize {
		return fmt.Errorf("incorrect DataHash size: %d != %d", len(s.DataHash), hashSize)
	}

	if len(s.AxisHash) != dahRootSize {
		return fmt.Errorf("incorrect AxisHash size: %d != %d", len(s.AxisHash), hashSize)
	}

	if s.Axis != rsmt2d.Col && s.Axis != rsmt2d.Row {
		return fmt.Errorf("incorrect Axis: %d", s.Axis)
	}

	return nil
}
