package ipldv2

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	ipldv2pb "github.com/celestiaorg/celestia-node/share/ipldv2/pb"
)

// SampleIDSize is the size of the SampleID in bytes
const SampleIDSize = 127

// TODO(@Wondertan): Eventually this should become configurable
const (
	hashSize     = sha256.Size
	dahRootSize  = 2*share.NamespaceSize + hashSize
	mhPrefixSize = 4
)

// SampleID is an unique identifier of a Sample.
type SampleID struct {
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

// NewSampleID constructs a new SampleID.
func NewSampleID(root *share.Root, idx int, axis rsmt2d.Axis) SampleID {
	sqrLn := len(root.RowRoots)
	row, col := idx/sqrLn, idx%sqrLn
	dahroot := root.RowRoots[row]
	if axis == rsmt2d.Col {
		dahroot = root.ColumnRoots[col]
	}

	return SampleID{
		DataHash: root.Hash(),
		AxisHash: dahroot,
		Index:    idx,
		Axis:     axis,
	}
}

// SampleIDFromCID coverts CID to SampleID.
func SampleIDFromCID(cid cid.Cid) (id SampleID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhalling SampleID: %w", err)
	}

	return id, nil
}

// Cid returns sample ID encoded as CID.
func (s *SampleID) Cid() (cid.Cid, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, multihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(codec, buf), nil
}

// Proto converts SampleID to its protobuf representation.
func (s *SampleID) Proto() *ipldv2pb.SampleID {
	return &ipldv2pb.SampleID{
		DataHash: s.DataHash,
		AxisHash: s.AxisHash,
		Index:    uint32(s.Index),
		Axis:     ipldv2pb.Axis(s.Axis),
	}
}

// MarshalBinary encodes SampleID into binary form.
func (s *SampleID) MarshalBinary() ([]byte, error) {
	// we cannot use protobuf here because it exceeds multihash limit of 128 bytes
	data := make([]byte, SampleIDSize)
	n := copy(data, s.DataHash)
	n += copy(data[n:], s.AxisHash)
	binary.LittleEndian.PutUint32(data[n:], uint32(s.Index))
	data[n+4] = byte(s.Axis)
	return data, nil
}

// UnmarshalBinary decodes SampleID from binary form.
func (s *SampleID) UnmarshalBinary(data []byte) error {
	if len(data) != SampleIDSize {
		return fmt.Errorf("incorrect sample id size: %d != %d", len(data), SampleIDSize)
	}

	// copying data to avoid slice aliasing
	s.DataHash = append(s.DataHash, data[:hashSize]...)
	s.AxisHash = append(s.AxisHash, data[hashSize:hashSize+dahRootSize]...)
	s.Index = int(binary.LittleEndian.Uint32(data[hashSize+dahRootSize : hashSize+dahRootSize+4]))
	s.Axis = rsmt2d.Axis(data[hashSize+dahRootSize+4])
	return nil
}

// Validate validates fields of SampleID.
func (s *SampleID) Validate() error {
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
