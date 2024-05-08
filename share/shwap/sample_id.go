package shwap

import (
	"encoding/binary"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// SampleIDSize defines the size of the SampleID in bytes, combining RowID size and 2 additional
// bytes for the ShareIndex.
const SampleIDSize = RowIDSize + 2

// SampleID uniquely identifies a specific sample within a row of an Extended Data Square (EDS).
type SampleID struct {
	RowID // Embeds RowID to incorporate block height and row index.

	// ShareIndex specifies the index of the sample within the row.
	ShareIndex uint16
}

// NewSampleID constructs a new SampleID using the provided block height, sample index, and a root
// structure for validation. It calculates the row and share index based on the sample index and
// the length of the row roots.
func NewSampleID(height uint64, smplIdx int, root *share.Root) (SampleID, error) {
	if root == nil || len(root.RowRoots) == 0 {
		return SampleID{}, fmt.Errorf("invalid root: root is nil or empty")
	}
	sqrLn := len(root.RowRoots)
	rowIdx, shrIdx := uint16(smplIdx/sqrLn), uint16(smplIdx%sqrLn)
	sid := SampleID{
		RowID: RowID{
			EdsID: EdsID{
				Height: height,
			},
			RowIndex: rowIdx,
		},
		ShareIndex: shrIdx,
	}

	if err := sid.Verify(root); err != nil {
		return SampleID{}, err
	}
	return sid, nil
}

// MarshalBinary encodes SampleID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (sid SampleID) MarshalBinary() []byte {
	data := make([]byte, 0, SampleIDSize)
	return sid.appendTo(data)
}

// SampleIDFromBinary deserializes a SampleID from binary data, ensuring the data length matches
// the expected size.
func SampleIDFromBinary(data []byte) (SampleID, error) {
	if len(data) != SampleIDSize {
		return SampleID{}, fmt.Errorf("invalid SampleID data length: expected %d, got %d", SampleIDSize, len(data))
	}

	rid, err := RowIDFromBinary(data[:RowIDSize])
	if err != nil {
		return SampleID{}, fmt.Errorf("error decoding RowID: %w", err)
	}

	shareIndex := binary.BigEndian.Uint16(data[RowIDSize:])
	return SampleID{RowID: rid, ShareIndex: shareIndex}, nil
}

// Verify checks the validity of the SampleID by ensuring the ShareIndex is within the bounds of
// the square size.
func (sid SampleID) Verify(root *share.Root) error {
	if err := sid.RowID.Verify(root); err != nil {
		return err
	}

	sqrLn := len(root.ColumnRoots) // Assumes ColumnRoots is valid and populated.
	if int(sid.ShareIndex) >= sqrLn {
		return fmt.Errorf("ShareIndex exceeds square size: %d >= %d", sid.ShareIndex, sqrLn)
	}

	return nil
}

// appendTo helps in constructing the binary representation by appending the encoded ShareIndex to
// the serialized RowID.
func (sid SampleID) appendTo(data []byte) []byte {
	data = sid.RowID.appendTo(data)
	return binary.BigEndian.AppendUint16(data, sid.ShareIndex)
}
