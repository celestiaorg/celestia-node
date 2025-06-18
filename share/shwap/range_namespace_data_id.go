package shwap

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RangeNamespaceDataIDSize defines the size of the RangeNamespaceDataIDSize in bytes,
// combining SampleID size, Namespace size, 4 additional bytes
// for the end coordinates of share of the range and uint representation of bool flag.
const RangeNamespaceDataIDSize = EdsIDSize + 8

// RangeNamespaceDataID uniquely identifies a continuous range of shares within a DataSquare (EDS)
// that belong to a specific namespace. The range is defined by the coordinates of the first (`From`)
// and last (`To`) shares in the range. This struct is used to reference and verify a subset of shares
// (e.g., for a blob or a namespace proof) within the EDS.
//
// Fields:
//   - NamespaceDataID: Embeds the EDS ID and the namespace identifier.
//   - From: The coordinates (row, col) of the first share in the range.
//   - To: The coordinates (row, col) of the last share in the range.
//
// Example usage:
//
//	id := RangeNamespaceDataID{
//	  NamespaceDataID: ...,
//	  From: shwap.SampleCoords{Row: 0, Col: 0},
//	  To:   shwap.SampleCoords{Row: 2, Col: 2},
//	}
type RangeNamespaceDataID struct {
	EdsID
	// From specifies the coordinates of the first share in the range.
	From SampleCoords
	// To specifies the coordinates of the last share in the range.
	To SampleCoords
}

func NewRangeNamespaceDataID(
	edsID EdsID,
	from SampleCoords,
	to SampleCoords,
	edsSize int,
) (RangeNamespaceDataID, error) {
	rngid := RangeNamespaceDataID{
		EdsID: edsID,
		From:  from,
		To:    to,
	}

	err := rngid.Verify(edsSize)
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("verifying range id: %w", err)
	}
	return rngid, nil
}

// Verify validates the RangeNamespaceDataID fields and verifies that number of the requested shares
// does not exceed the number of shares inside the ODS.
func (rngid RangeNamespaceDataID) Verify(edsSize int) error {
	err := rngid.EdsID.Validate()
	if err != nil {
		return fmt.Errorf("invalid EdsID: %w", err)
	}
	fromIdx, err := SampleCoordsAs1DIndex(rngid.From, edsSize)
	if err != nil {
		return err
	}
	// verify that to is not exceed that edsSize
	toIdx, err := SampleCoordsAs1DIndex(rngid.To, edsSize)
	if err != nil {
		return err
	}
	if fromIdx > toIdx {
		return fmt.Errorf("invalid range: from index %d > to index %d", fromIdx, toIdx)
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
	return rngid.EdsID == other.EdsID && rngid.From == other.From &&
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

	fromCoords := SampleCoords{
		Row: int(binary.BigEndian.Uint16(data[EdsIDSize : EdsIDSize+2])),
		Col: int(binary.BigEndian.Uint16(data[EdsIDSize+2 : EdsIDSize+4])),
	}
	toCoords := SampleCoords{
		Row: int(binary.BigEndian.Uint16(data[EdsIDSize+4 : EdsIDSize+6])),
		Col: int(binary.BigEndian.Uint16(data[EdsIDSize+6 : EdsIDSize+8])),
	}

	rngID := RangeNamespaceDataID{
		EdsID: edsID,
		From:  fromCoords,
		To:    toCoords,
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
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.From.Row))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.From.Col))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To.Row))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To.Col))
	return data, nil
}
