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
const ShareSampleIDSize = 127

// ShareSampleID is an unique identifier of a ShareSample.
type ShareSampleID struct {
	// DataHash is the root of the data square
	// Needed to identify the data square in the whole chain
	DataHash share.DataHash
	// AxisHash is the Col or Row root from DAH of the data square
	AxisHash []byte
	// Index is the index of the sample in the data square(not row or col index)
	Index int
	// Axis is Col or Row axis of the sample in the data square
	Axis rsmt2d.Axis
}

// NewShareSampleID constructs a new ShareSampleID.
func NewShareSampleID(root *share.Root, idx int, axis rsmt2d.Axis) ShareSampleID {
	sqrLn := len(root.RowRoots)
	row, col := idx/sqrLn, idx%sqrLn
	dahroot := root.RowRoots[row]
	if axis == rsmt2d.Col {
		dahroot = root.ColumnRoots[col]
	}

	return ShareSampleID{
		DataHash: root.Hash(),
		AxisHash: dahroot,
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
		DataHash: s.DataHash,
		AxisHash: s.AxisHash,
		Index:    uint32(s.Index),
		Axis:     ipldv2pb.Axis(s.Axis),
	}
}

// MarshalBinary encodes ShareSampleID into binary form.
func (s *ShareSampleID) MarshalBinary() ([]byte, error) {
	// we cannot use protobuf here because it exceeds multihash limit of 128 bytes
	data := make([]byte, ShareSampleIDSize)
	n := copy(data, s.DataHash)
	n += copy(data[n:], s.AxisHash)
	binary.LittleEndian.PutUint32(data[n:], uint32(s.Index))
	data[n+4] = byte(s.Axis)
	return data, nil
}

// UnmarshalBinary decodes ShareSampleID from binary form.
func (s *ShareSampleID) UnmarshalBinary(data []byte) error {
	if len(data) != ShareSampleIDSize {
		return fmt.Errorf("incorrect SampleID size: %d != %d", len(data), ShareSampleIDSize)
	}

	// copying data to avoid slice aliasing
	s.DataHash = append(s.DataHash, data[:hashSize]...)
	s.AxisHash = append(s.AxisHash, data[hashSize:hashSize+dahRootSize]...)
	s.Index = int(binary.LittleEndian.Uint32(data[hashSize+dahRootSize : hashSize+dahRootSize+4]))
	s.Axis = rsmt2d.Axis(data[hashSize+dahRootSize+4])
	return nil
}

// Validate validates fields of ShareSampleID.
func (s *ShareSampleID) Validate() error {
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
