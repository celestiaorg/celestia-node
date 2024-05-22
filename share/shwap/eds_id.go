package shwap

import (
	"encoding/binary"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// EdsIDSize defines the byte size of the EdsID.
const EdsIDSize = 8

// EdsID represents a unique identifier for a row, using the height of the block
// to identify the data square in the chain.
type EdsID struct {
	Height uint64 // Height specifies the block height.
}

// NewEdsID creates a new EdsID using the given height and verifies it against the provided Root.
// It returns an error if the verification fails.
func NewEdsID(height uint64, root *share.Root) (EdsID, error) {
	eid := EdsID{
		Height: height,
	}
	return eid, eid.Verify(root)
}

// MarshalBinary encodes an EdsID into its binary form, primarily for storage or network
// transmission.
func (eid EdsID) MarshalBinary() []byte {
	data := make([]byte, 0, EdsIDSize)
	return eid.appendTo(data)
}

// EdsIDFromBinary decodes a byte slice into an EdsID, validating the length of the data.
// It returns an error if the data slice does not match the expected size of an EdsID.
func EdsIDFromBinary(data []byte) (EdsID, error) {
	if len(data) != EdsIDSize {
		return EdsID{}, fmt.Errorf("invalid EdsID data length: %d != %d", len(data), EdsIDSize)
	}
	rid := EdsID{
		Height: binary.BigEndian.Uint64(data),
	}
	return rid, nil
}

// Verify checks the integrity of an EdsID's fields against the provided Root.
// It ensures that the EdsID is not constructed with a zero Height and that the root is not nil.
func (eid EdsID) Verify(root *share.Root) error {
	if root == nil {
		return fmt.Errorf("provided Root is nil")
	}
	if eid.Height == 0 {
		return fmt.Errorf("height cannot be zero")
	}
	return nil
}

// GetHeight returns the Height of the EdsID.
func (eid EdsID) GetHeight() uint64 {
	return eid.Height
}

// appendTo helps in the binary encoding of EdsID by appending the binary form of Height to the
// given byte slice.
func (eid EdsID) appendTo(data []byte) []byte {
	return binary.BigEndian.AppendUint64(data, eid.Height)
}
