package ipldv2

import (
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// SampleIDSize is the size of the SampleID in bytes
const SampleIDSize = AxisIDSize + 2

// SampleID is an unique identifier of a Sample.
type SampleID struct {
	AxisID

	// ShareIndex is the index of the sampled share in the axis
	ShareIndex uint16
}

// NewSampleID constructs a new SampleID.
func NewSampleID(axisType rsmt2d.Axis, idx int, root *share.Root, height uint64) SampleID {
	sqrLn := len(root.RowRoots)
	axisIdx, shrIdx := idx/sqrLn, idx%sqrLn
	dahroot := root.RowRoots[axisIdx]
	if axisType == rsmt2d.Col {
		axisIdx, shrIdx = shrIdx, axisIdx
		dahroot = root.ColumnRoots[axisIdx]
	}
	axisHash := hashBytes(dahroot)

	return SampleID{
		AxisID: AxisID{
			AxisType:  axisType,
			AxisIndex: uint16(axisIdx),
			AxisHash:  axisHash,
			Height:    height,
		},
		ShareIndex: uint16(shrIdx),
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
func (s SampleID) Cid() (cid.Cid, error) {
	// avoid using proto serialization for CID as it's not deterministic
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, sampleMultihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(sampleCodec, buf), nil
}

// MarshalBinary encodes SampleID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (s SampleID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, SampleIDSize)
	n, err := s.AxisID.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	data = data[:n]
	data = binary.LittleEndian.AppendUint16(data, s.ShareIndex)
	return data, nil
}

// UnmarshalBinary decodes SampleID from binary form.
func (s *SampleID) UnmarshalBinary(data []byte) error {
	if len(data) != SampleIDSize {
		return fmt.Errorf("invalid data length: %d != %d", len(data), SampleIDSize)
	}
	n, err := s.AxisID.UnmarshalFrom(data)
	if err != nil {
		return err
	}
	s.ShareIndex = binary.LittleEndian.Uint16(data[n:])
	return nil
}

// Validate validates fields of SampleID.
func (s SampleID) Validate() error {
	return s.AxisID.Validate()
}
