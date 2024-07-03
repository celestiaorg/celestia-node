package shwap

import (
	"encoding/binary"
	"fmt"
)

// SampleIDSize defines the size of the SampleID in bytes, combining RowID size and 2 additional
// bytes for the ShareIndex.
const SampleIDSize = RowIDSize + 2

// SampleID uniquely identifies a specific sample within a row of an Extended Data Square (EDS).
type SampleID struct {
	RowID          // Embeds RowID to incorporate block height and row index.
	ShareIndex int // ShareIndex specifies the index of the sample within the row.
}

// NewSampleID constructs a new SampleID using the provided block height, sample index, and EDS
// size. It calculates the row and share index based on the sample index and EDS size.
func NewSampleID(height uint64, rowIdx, colIdx, edsSize int) (SampleID, error) {
	sid := SampleID{
		RowID: RowID{
			EdsID: EdsID{
				Height: height,
			},
			RowIndex: rowIdx,
		},
		ShareIndex: colIdx,
	}

	if err := sid.Verify(edsSize); err != nil {
		return SampleID{}, fmt.Errorf("verifying SampleID: %w", err)
	}
	return sid, nil
}

// SampleIDFromBinary deserializes a SampleID from binary data, ensuring the data length matches
// the expected size.
func SampleIDFromBinary(data []byte) (SampleID, error) {
	if len(data) != SampleIDSize {
		return SampleID{}, fmt.Errorf("invalid SampleID data length: expected %d, got %d", SampleIDSize, len(data))
	}

	rid, err := RowIDFromBinary(data[:RowIDSize])
	if err != nil {
		return SampleID{}, fmt.Errorf("decoding RowID: %w", err)
	}

	sid := SampleID{
		RowID:      rid,
		ShareIndex: int(binary.BigEndian.Uint16(data[RowIDSize:])),
	}
	if err := sid.Validate(); err != nil {
		return SampleID{}, fmt.Errorf("validating SampleID: %w", err)
	}

	return sid, nil
}

// MarshalBinary encodes SampleID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (sid SampleID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, SampleIDSize)
	return sid.appendTo(data), nil
}

// Verify validates the SampleID and verifies that the ShareIndex is within the bounds of
// the square size.
func (sid SampleID) Verify(edsSize int) error {
	if err := sid.RowID.Verify(edsSize); err != nil {
		return fmt.Errorf("verifying RowID: %w", err)
	}
	if sid.ShareIndex >= edsSize {
		return fmt.Errorf("%w: ShareIndex: %d >= %d", ErrOutOfBounds, sid.ShareIndex, edsSize)
	}
	return sid.Validate()
}

// Validate performs basic field validation.
func (sid SampleID) Validate() error {
	if sid.ShareIndex < 0 {
		return fmt.Errorf("%w: ShareIndex: %d < 0", ErrInvalidShwapID, sid.ShareIndex)
	}
	return sid.RowID.Validate()
}

// appendTo helps in constructing the binary representation by appending the encoded ShareIndex to
// the serialized RowID.
func (sid SampleID) appendTo(data []byte) []byte {
	data = sid.RowID.appendTo(data)
	return binary.BigEndian.AppendUint16(data, uint16(sid.ShareIndex))
}
