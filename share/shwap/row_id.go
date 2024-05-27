package shwap

import (
	"encoding/binary"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// RowIDSize defines the size in bytes of RowID, consisting of the size of EdsID and 2 bytes for
// RowIndex.
const RowIDSize = EdsIDSize + 2

// RowID uniquely identifies a row in the data square of a blockchain block, combining block height
// with the row's index.
type RowID struct {
	EdsID        // Embedding EdsID to include the block height in RowID.
	RowIndex int // RowIndex specifies the position of the row within the data square.
}

// NewRowID creates a new RowID with the specified block height, row index, and validates it
// against the provided Root. It returns an error if the validation fails, ensuring the RowID
// conforms to expected constraints.
func NewRowID(height uint64, rowIdx int, root *share.Root) (RowID, error) {
	rid := RowID{
		EdsID: EdsID{
			Height: height,
		},
		RowIndex: rowIdx,
	}
	return rid, rid.Validate(root)
}

// MarshalBinary encodes the RowID into a binary form for storage or network transmission.
func (rid RowID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RowIDSize)
	return rid.appendTo(data), nil
}

// RowIDFromBinary decodes a RowID from its binary representation.
// It returns an error if the input data does not conform to the expected size or content format.
func RowIDFromBinary(data []byte) (RowID, error) {
	if len(data) != RowIDSize {
		return RowID{}, fmt.Errorf("invalid RowID data length: expected %d, got %d", RowIDSize, len(data))
	}
	eid, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return RowID{}, fmt.Errorf("error decoding EdsID: %w", err)
	}
	return RowID{
		EdsID:    eid,
		RowIndex: int(binary.BigEndian.Uint16(data[EdsIDSize:])),
	}, nil
}

// Validate ensures the RowID's fields are valid given the specified root structure, particularly
// that the row index is within bounds.
func (rid RowID) Validate(root *share.Root) error {
	if err := rid.EdsID.Validate(root); err != nil {
		return err
	}

	if root == nil || len(root.RowRoots) == 0 {
		return fmt.Errorf("provided root is nil or empty")
	}

	if rid.RowIndex >= len(root.RowRoots) {
		return fmt.Errorf("RowIndex out of bounds: %d >= %d", rid.RowIndex, len(root.RowRoots))
	}

	return nil
}

// appendTo assists in binary encoding of RowID by appending the encoded fields to the given byte
// slice.
func (rid RowID) appendTo(data []byte) []byte {
	data = rid.EdsID.appendTo(data)
	return binary.BigEndian.AppendUint16(data, uint16(rid.RowIndex))
}
