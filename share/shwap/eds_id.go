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

// MarshalTo encodes EdsID into given byte slice.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (rid EdsID) MarshalTo(data []byte) (int, error) {
	// TODO:(@walldiss): this works, only if data underlying array was preallocated with
	//  enough size. Otherwise Caller might not see the changes.
	data = binary.BigEndian.AppendUint64(data, rid.Height)
	return EdsIDSize, nil
}

// UnmarshalFrom decodes EdsID from given byte slice.
func (rid *EdsID) UnmarshalFrom(data []byte) (int, error) {
	rid.Height = binary.BigEndian.Uint64(data)
	return EdsIDSize, nil
}

// MarshalBinary encodes EdsID into binary form.
func (rid EdsID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, EdsIDSize)
	n, err := rid.MarshalTo(data)
	return data[:n], err
}

// UnmarshalBinary decodes EdsID from binary form.
func (rid *EdsID) UnmarshalBinary(data []byte) error {
	if len(data) != EdsIDSize {
		return fmt.Errorf("invalid EdsID data length: %d != %d", len(data), EdsIDSize)
	}
	_, err := rid.UnmarshalFrom(data)
	return err
}

// Verify verifies EdsID fields.
func (rid EdsID) Verify(root *share.Root) error {
	if root == nil {
		return fmt.Errorf("nil Root")
	}
	if rid.Height == 0 {
		return fmt.Errorf("zero Height")
	}

	return nil
}

func (rid EdsID) GetHeight() uint64 {
	return rid.Height
}
