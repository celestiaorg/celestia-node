package shwap

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/celestiaorg/rsmt2d"
)

// RowIDSize defines the size in bytes of RowID, consisting of the size of EdsID and 2 bytes for
// RowIndex.
const RowIDSize = EdsIDSize + 2

// RowID uniquely identifies a row in the data square of a blockchain block, combining block height
// with the row's index.
type RowID struct {
	EdsID        // Embedding EdsID to include the block height in RowID.
	RowIndex int // RowIndex specifies the position of the row within the data share.
}

// NewRowID creates a new RowID with the specified block height, row index, and EDS size.
// It returns an error if the validation fails, ensuring the RowID
// conforms to expected constraints.
func NewRowID(height uint64, rowIdx, edsSize int) (RowID, error) {
	rid := RowID{
		EdsID: EdsID{
			height: height,
		},
		RowIndex: rowIdx,
	}
	if err := rid.Verify(edsSize); err != nil {
		return RowID{}, fmt.Errorf("verifying RowID: %w", err)
	}

	return rid, nil
}

// RowIDFromBinary decodes a RowID from its binary representation.
// It returns an error if the input data does not conform to the expected size or content format.
func RowIDFromBinary(data []byte) (RowID, error) {
	if len(data) != RowIDSize {
		return RowID{}, fmt.Errorf("invalid RowID data length: expected %d, got %d", RowIDSize, len(data))
	}
	eid, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return RowID{}, fmt.Errorf("decoding EdsID: %w", err)
	}

	rid := RowID{
		EdsID:    eid,
		RowIndex: int(binary.BigEndian.Uint16(data[EdsIDSize:])),
	}
	if err := rid.Validate(); err != nil {
		return RowID{}, fmt.Errorf("validating RowID: %w", err)
	}

	return rid, nil
}

// Equals checks equality of RowID.
func (rid *RowID) Equals(other RowID) bool {
	return rid.EdsID.Equals(other.EdsID) && rid.RowIndex == other.RowIndex
}

// ReadFrom reads the binary form of RowID from the provided reader.
func (rid *RowID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, RowIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}
	if n != RowIDSize {
		return int64(n), fmt.Errorf("RowID: expected %d bytes, got %d", RowIDSize, n)
	}
	id, err := RowIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("RowIDFromBinary: %w", err)
	}
	*rid = id
	return int64(n), nil
}

// MarshalBinary encodes the RowID into a binary form for storage or network transmission.
func (rid RowID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RowIDSize)
	data, err := rid.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes the binary form of RowID to the provided writer.
func (rid RowID) WriteTo(w io.Writer) (int64, error) {
	data, err := rid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Verify validates the RowID fields and verifies that RowIndex is within the bounds of
// the square size
func (rid RowID) Verify(edsSize int) error {
	if edsSize == 0 {
		return fmt.Errorf("provided EDS size is zero")
	}

	if rid.RowIndex >= edsSize {
		return fmt.Errorf("%w, RowIndex: %d >= %d", ErrOutOfBounds, rid.RowIndex, edsSize)
	}

	return rid.Validate()
}

// Validate performs basic field validation.
func (rid RowID) Validate() error {
	if rid.RowIndex < 0 {
		return fmt.Errorf("%w: RowIndex: %d < 0", ErrInvalidID, rid.RowIndex)
	}
	return rid.EdsID.Validate()
}

// AppendBinary assists in binary encoding of RowID by appending the encoded fields to the given byte
// slice.
func (rid RowID) AppendBinary(data []byte) ([]byte, error) {
	data, err := rid.EdsID.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return binary.BigEndian.AppendUint16(data, uint16(rid.RowIndex)), nil
}

func (rid RowID) ContainerDataReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	axisHalf, err := acc.AxisHalf(ctx, rsmt2d.Row, rid.RowIndex)
	if err != nil {
		return nil, err
	}

	r := axisHalf.ToRow()
	buf := &bytes.Buffer{}
	_, err = r.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
