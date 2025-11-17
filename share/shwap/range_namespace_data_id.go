package shwap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// RangeNamespaceDataIDSize defines the size of the RangeNamespaceDataIDSize in bytes,
// combining EdsIDSize size and 4 additional bytes
// for the start and end ODS indexes of share of the range.
const RangeNamespaceDataIDSize = EdsIDSize + 8

// RangeNamespaceDataID uniquely identifies a continuous range of shares within an Original DataSquare (ODS)
// The range is defined by the indexes of the first (`From`)
// and last (`To`) (exclusively) shares in the range. This struct is used to reference and verify a subset of shares
// (e.g., for a blob or a namespace proof) within the ODS.
//
// Fields:
//   - EdsID: to identify the height
//   - From: The index of the first share in the range.
//   - To: The index of the last share in the range(exclusively).
//
// Example usage:
//
//	id := RangeNamespaceDataID{
//	  EdsID: ...,
//	  From: 0,
//	  To:   4,
//	}
type RangeNamespaceDataID struct {
	EdsID
	// From specifies the index of the first share in the range.
	From uint32
	// To specifies the index of the last share in the range(exclusively).
	To uint32
}

func NewRangeNamespaceDataID(
	edsID EdsID,
	from, to, odsSize int,
) (RangeNamespaceDataID, error) {
	if from < 0 || to < 0 {
		return RangeNamespaceDataID{}, errors.New("invalid range: failed to build range with negative indexes")
	}
	rngid := RangeNamespaceDataID{
		EdsID: edsID,
		From:  uint32(from),
		To:    uint32(to),
	}

	err := rngid.Verify(odsSize)
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("verifying range id: %w", err)
	}
	return rngid, nil
}

// Verify validates the RangeNamespaceDataID fields and verifies that number of the requested shares
// does not exceed the number of shares inside the ODS.
func (rngid RangeNamespaceDataID) Verify(odsSize int) error {
	err := rngid.EdsID.Validate()
	if err != nil {
		return fmt.Errorf("invalid EdsID: %w", err)
	}

	sharesAmount := uint32(odsSize * odsSize)

	if rngid.To == uint32(0) {
		return fmt.Errorf("invalid range: to must be greater than 0: %d", rngid.To)
	}
	if rngid.From >= rngid.To {
		return fmt.Errorf("invalid range: from %d to %d", rngid.From, rngid.To)
	}
	if rngid.From >= sharesAmount {
		return fmt.Errorf("invalid start index: from %d >= size: %d", rngid.From, odsSize)
	}
	if rngid.To > sharesAmount {
		return fmt.Errorf("invalid end index: to %d > size: %d", rngid.To, odsSize)
	}
	return nil
}

// Validate performs basic fields validation.
func (rngid RangeNamespaceDataID) Validate() error {
	return nil
}

// ReadFrom reads the binary form of RangeNamespaceDataID from the provided reader.
func (rngid *RangeNamespaceDataID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, RangeNamespaceDataIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}

	id, err := RangeNamespaceDataIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("RangeNamespaceDataIDFromBinary: %w", err)
	}
	*rngid = id
	return int64(n), nil
}

// WriteTo writes the binary form of RangeNamespaceDataID to the provided writer.
func (rngid RangeNamespaceDataID) WriteTo(w io.Writer) (int64, error) {
	data, err := rngid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Equals checks equality of RangeNamespaceDataID.
func (rngid *RangeNamespaceDataID) Equals(other RangeNamespaceDataID) bool {
	return rngid.EdsID.Equals(other.EdsID) && rngid.From == other.From &&
		rngid.To == other.To
}

// RangeNamespaceDataIDFromBinary deserializes a RangeNamespaceDataID from its binary form.
func RangeNamespaceDataIDFromBinary(data []byte) (RangeNamespaceDataID, error) {
	if len(data) != RangeNamespaceDataIDSize {
		return RangeNamespaceDataID{}, fmt.Errorf(
			"invalid RangeNamespaceDataID data length: expected %d, got %d", RangeNamespaceDataIDSize, len(data),
		)
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return RangeNamespaceDataID{}, err
	}

	rngID := RangeNamespaceDataID{
		EdsID: edsID,
		From:  binary.BigEndian.Uint32(data[EdsIDSize : EdsIDSize+4]),
		To:    binary.BigEndian.Uint32(data[EdsIDSize+4 : EdsIDSize+8]),
	}
	return rngID, rngID.Validate()
}

// MarshalBinary encodes RangeNamespaceDataID into binary form.
func (rngid RangeNamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RangeNamespaceDataIDSize)
	return rngid.appendTo(data)
}

// appendTo helps in constructing the binary representation of RangeNamespaceDataID
// by appending all encoded fields.
func (rngid RangeNamespaceDataID) appendTo(data []byte) ([]byte, error) {
	data, err := rngid.AppendBinary(data)
	if err != nil {
		return nil, fmt.Errorf("appending EdsID: %w", err)
	}
	data = binary.BigEndian.AppendUint32(data, rngid.From)
	data = binary.BigEndian.AppendUint32(data, rngid.To)
	return data, nil
}
