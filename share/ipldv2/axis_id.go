package ipldv2

import (
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// AxisIDSize is the size of the AxisID in bytes
const AxisIDSize = 43

// AxisID is an unique identifier of a Axis.
type AxisID struct {
	// AxisType is Col or Row axis of the sample in the data square
	AxisType rsmt2d.Axis
	// AxisIndex is the index of the axis(row, col) in the data square
	AxisIndex uint16
	// AxisHash is the sha256 hash of a Col or Row root taken from DAH of the data square
	AxisHash []byte
	// Height of the block.
	// Needed to identify block's data square in the whole chain
	Height uint64
}

// NewAxisID constructs a new AxisID.
func NewAxisID(axisType rsmt2d.Axis, axisIdx uint16, root *share.Root, height uint64) AxisID {
	dahroot := root.RowRoots[axisIdx]
	if axisType == rsmt2d.Col {
		dahroot = root.ColumnRoots[axisIdx]
	}
	axisHash := hashBytes(dahroot)

	return AxisID{
		AxisType:  axisType,
		AxisIndex: axisIdx,
		AxisHash:  axisHash,
		Height:    height,
	}
}

// AxisIDFromCID coverts CID to AxisID.
func AxisIDFromCID(cid cid.Cid) (id AxisID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhalling AxisID: %w", err)
	}

	return id, nil
}

// Cid returns sample ID encoded as CID.
func (s *AxisID) Cid() (cid.Cid, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, axisMultihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(axisCodec, buf), nil
}

// MarshalTo encodes AxisID into given byte slice.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (s *AxisID) MarshalTo(data []byte) (int, error) {
	data = append(data, byte(s.AxisType))
	data = binary.LittleEndian.AppendUint16(data, s.AxisIndex)
	data = append(data, s.AxisHash...)
	binary.LittleEndian.AppendUint64(data, s.Height)
	return AxisIDSize, nil
}

// UnmarshalFrom decodes AxisID from given byte slice.
func (s *AxisID) UnmarshalFrom(data []byte) (int, error) {
	s.AxisType = rsmt2d.Axis(data[0])
	s.AxisIndex = binary.LittleEndian.Uint16(data[1:])
	s.AxisHash = append(s.AxisHash, data[3:hashSize+3]...)
	s.Height = binary.LittleEndian.Uint64(data[hashSize+3:])
	return AxisIDSize, nil
}

// MarshalBinary encodes AxisID into binary form.
func (s *AxisID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, AxisIDSize)
	n, err := s.MarshalTo(data)
	return data[:n], err
}

// UnmarshalBinary decodes AxisID from binary form.
func (s *AxisID) UnmarshalBinary(data []byte) error {
	if len(data) != AxisIDSize {
		return fmt.Errorf("invalid data length: %d != %d", len(data), AxisIDSize)
	}
	_, err := s.UnmarshalFrom(data)
	return err
}

// Validate validates fields of AxisID.
func (s *AxisID) Validate() error {
	if s.Height == 0 {
		return fmt.Errorf("zero Height")
	}

	if len(s.AxisHash) != hashSize {
		return fmt.Errorf("invalid AxisHash size: %d != %d", len(s.AxisHash), hashSize)
	}

	if s.AxisType != rsmt2d.Col && s.AxisType != rsmt2d.Row {
		return fmt.Errorf("invalid AxisType: %d", s.AxisType)
	}

	return nil
}
