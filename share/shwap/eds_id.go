package shwap

import (
	"encoding/binary"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// EdsIDSize is the size of the EdsID in bytes
const EdsIDSize = 8

// EdsID is an unique identifier of a Row.
type EdsID struct {
	// Height of the block.
	// Needed to identify block's data square in the whole chain
	Height uint64
}

// NewEdsID constructs a new EdsID.
func NewEdsID(height uint64, root *share.Root) (EdsID, error) {
	rid := EdsID{
		Height: height,
	}
	return rid, rid.Verify(root)
}

// MarshalBinary encodes EdsID into binary form.
func (eid EdsID) MarshalBinary() []byte {
	data := make([]byte, 0, EdsIDSize)
	return eid.appendTo(data)
}

// EdsIDFromBinary decodes EdsID from binary form.
func EdsIDFromBinary(data []byte) (rid EdsID, err error) {
	if len(data) != EdsIDSize {
		return rid, fmt.Errorf("invalid EdsID data length: %d != %d", len(data), EdsIDSize)
	}
	rid.Height = binary.BigEndian.Uint64(data)
	return rid, nil
}

// Verify verifies EdsID fields.
func (eid EdsID) Verify(root *share.Root) error {
	if root == nil {
		return fmt.Errorf("nil Root")
	}
	if eid.Height == 0 {
		return fmt.Errorf("zero Height")
	}

	return nil
}

func (eid EdsID) GetHeight() uint64 {
	return eid.Height
}

func (eid EdsID) appendTo(data []byte) []byte {
	return binary.BigEndian.AppendUint64(data, eid.Height)
}
