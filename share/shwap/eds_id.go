package shwap

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

// EdsIDSize defines the byte size of the EdsID.
const EdsIDSize = 8

// EdsID represents a unique identifier for a row, using the height of the block
// to identify the data square in the chain.
type EdsID struct {
	Height uint64 // Height specifies the block height.
}

// NewEdsID creates a new EdsID using the given height.
func NewEdsID(height uint64) (EdsID, error) {
	eid := EdsID{
		Height: height,
	}
	return eid, eid.Validate()
}

// EdsIDFromBinary decodes a byte slice into an EdsID, validating the length of the data.
// It returns an error if the data slice does not match the expected size of an EdsID.
func EdsIDFromBinary(data []byte) (EdsID, error) {
	if len(data) != EdsIDSize {
		return EdsID{}, fmt.Errorf("invalid EdsID data length: %d != %d", len(data), EdsIDSize)
	}
	eid := EdsID{
		Height: binary.BigEndian.Uint64(data),
	}
	if err := eid.Validate(); err != nil {
		return EdsID{}, fmt.Errorf("validating EdsID: %w", err)
	}

	return eid, nil
}

func (eid EdsID) Name() string {
	return edsName
}

// Equals checks equality of EdsIDs.
func (eid *EdsID) Equals(other EdsID) bool {
	return eid.Height == other.Height
}

// ReadFrom reads the binary form of EdsID from the provided reader.
func (eid *EdsID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, EdsIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}
	if n != EdsIDSize {
		return int64(n), fmt.Errorf("EdsID: expected %d bytes, got %d", EdsIDSize, n)
	}
	id, err := EdsIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("EdsIDFromBinary: %w", err)
	}
	*eid = id
	return int64(n), nil
}

// MarshalBinary encodes an EdsID into its binary form, primarily for storage or network
// transmission.
func (eid EdsID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, EdsIDSize)
	data, err := eid.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes the binary form of EdsID to the provided writer.
func (eid EdsID) WriteTo(w io.Writer) (int64, error) {
	data, err := eid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Validate checks the integrity of an EdsID's fields against the provided Root.
// It ensures that the EdsID is not constructed with a zero Height and that the root is not nil.
func (eid EdsID) Validate() error {
	if eid.Height == 0 {
		return fmt.Errorf("%w: Height == 0", ErrInvalidID)
	}
	return nil
}

func (eid EdsID) Target() uint64 {
	return eid.Height
}

func (eid EdsID) FetchContainerReader(_ context.Context, acc Accessor) (io.Reader, error) {
	return acc.Reader()
}

// AppendBinary helps in the binary encoding of EdsID by appending the binary form of Height to the
// given byte slice.
func (eid EdsID) AppendBinary(data []byte) ([]byte, error) {
	return binary.BigEndian.AppendUint64(data, eid.Height), nil
}
