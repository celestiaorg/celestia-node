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
	EdsID // Embedding EdsID to include the block height in RowID.

	RowIndex uint16 // RowIndex specifies the position of the row within the data square.
}

// NewRowID creates a new RowID with the specified block height, row index, and validates it
// against the provided Root. It returns an error if the validation fails, ensuring the RowID
// conforms to expected constraints.
func NewRowID(height uint64, rowIdx uint16, root *share.Root) (RowID, error) {
	rid := RowID{
		EdsID: EdsID{
			Height: height,
		},
		RowIndex: rowIdx,
	}
	return rid, rid.Verify(root)
}

// MarshalBinary encodes the RowID into a binary form for storage or network transmission.
func (rid RowID) MarshalBinary() []byte {
	data := make([]byte, 0, RowIDSize)
	return rid.appendTo(data)
}

// RowIDFromBinary decodes a RowID from its binary representation.
// It returns an error if the input data does not conform to the expected size or content format.
func RowIDFromBinary(data []byte) (RowID, error) {
	var rid RowID
	if len(data) != RowIDSize {
		return rid, fmt.Errorf("invalid RowID data length: expected %d, got %d", RowIDSize, len(data))
	}
	eid, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return rid, fmt.Errorf("error decoding EdsID: %w", err)
	}
	rid.EdsID = eid
	rid.RowIndex = binary.BigEndian.Uint16(data[EdsIDSize:])
	return rid, nil
}

// Verify ensures the RowID's fields are valid given the specified root structure, particularly
// that the row index is within bounds.
func (rid RowID) Verify(root *share.Root) error {
	if err := rid.EdsID.Verify(root); err != nil {
		return err
	}

	if root == nil || len(root.RowRoots) == 0 {
		return fmt.Errorf("provided root is nil or empty")
	}

	if int(rid.RowIndex) >= len(root.RowRoots) {
		return fmt.Errorf("RowIndex out of bounds: %d >= %d", rid.RowIndex, len(root.RowRoots))
	}

	return nil
}

// appendTo assists in binary encoding of RowID by appending the encoded fields to the given byte
// slice.
func (rid RowID) appendTo(data []byte) []byte {
	data = rid.EdsID.appendTo(data)
	return binary.BigEndian.AppendUint16(data, rid.RowIndex)
}
