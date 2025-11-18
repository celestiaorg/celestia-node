package shwap

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RangeNamespaceDataIDV1Size defines the size of the RangeNamespaceDataIDV1Size in bytes,
// combining EdsIDSize size and 8 additional bytes
// for the start and end ODS indexes of share of the range.
const RangeNamespaceDataIDV1Size = EdsIDSize + 8

// RangeNamespaceDataIDV1 uniquely identifies a continuous range of shares within an Original DataSquare (ODS)
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
//	id := RangeNamespaceDataIDV1{
//	  EdsID: ...,
//	  From: 0,
//	  To:   4,
//	}
type RangeNamespaceDataIDV1 struct {
	EdsID
	// From specifies the index of the first share in the range.
	From int
	// To specifies the index of the last share in the range(exclusively).
	To int
}

func NewRangeNamespaceDataIDV1(
	edsID EdsID,
	from, to, odsSize int,
) (RangeNamespaceDataIDV1, error) {
	rngid := RangeNamespaceDataIDV1{
		EdsID: edsID,
		From:  from,
		To:    to,
	}

	err := rngid.Verify(odsSize)
	if err != nil {
		return RangeNamespaceDataIDV1{}, fmt.Errorf("verifying range id: %w", err)
	}
	return rngid, nil
}

// Verify validates the RangeNamespaceDataIDV1 fields and verifies that number of the requested shares
// does not exceed the number of shares inside the ODS.
func (rngid RangeNamespaceDataIDV1) Verify(odsSize int) error {
	err := rngid.EdsID.Validate()
	if err != nil {
		return fmt.Errorf("invalid EdsID: %w", err)
	}

	sharesAmount := odsSize * odsSize
	if rngid.From < 0 {
		return fmt.Errorf("from must be greater than or equal to 0: %d", rngid.From)
	}
	if rngid.To <= 0 {
		return fmt.Errorf("to must be greater than 0: %d", rngid.To)
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
func (rngid RangeNamespaceDataIDV1) Validate() error {
	return nil
}

// ReadFrom reads the binary form of RangeNamespaceDataIDV1 from the provided reader.
func (rngid *RangeNamespaceDataIDV1) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, RangeNamespaceDataIDV1Size)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}

	id, err := RangeNamespaceDataIDV1FromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("RangeNamespaceDataIDV1FromBinary: %w", err)
	}
	*rngid = id
	return int64(n), nil
}

// WriteTo writes the binary form of RangeNamespaceDataIDV1 to the provided writer.
func (rngid RangeNamespaceDataIDV1) WriteTo(w io.Writer) (int64, error) {
	data, err := rngid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Equals checks equality of RangeNamespaceDataIDV1.
func (rngid *RangeNamespaceDataIDV1) Equals(other RangeNamespaceDataIDV1) bool {
	return rngid.EdsID.Equals(other.EdsID) && rngid.From == other.From &&
		rngid.To == other.To
}

// RangeNamespaceDataIDV1FromBinary deserializes a RangeNamespaceDataIDV1 from its binary form.
func RangeNamespaceDataIDV1FromBinary(data []byte) (RangeNamespaceDataIDV1, error) {
	if len(data) != RangeNamespaceDataIDV1Size {
		return RangeNamespaceDataIDV1{}, fmt.Errorf(
			"invalid RangeNamespaceDataIDV1 data length: expected %d, got %d", RangeNamespaceDataIDV1Size, len(data),
		)
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return RangeNamespaceDataIDV1{}, err
	}

	rngID := RangeNamespaceDataIDV1{
		EdsID: edsID,
		From:  int(binary.BigEndian.Uint32(data[EdsIDSize : EdsIDSize+4])),
		To:    int(binary.BigEndian.Uint32(data[EdsIDSize+4 : EdsIDSize+8])),
	}
	return rngID, rngID.Validate()
}

// MarshalBinary encodes RangeNamespaceDataIDV1 into binary form.
func (rngid RangeNamespaceDataIDV1) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RangeNamespaceDataIDV1Size)
	return rngid.appendTo(data)
}

// appendTo helps in constructing the binary representation of RangeNamespaceDataIDV1
// by appending all encoded fields.
func (rngid RangeNamespaceDataIDV1) appendTo(data []byte) ([]byte, error) {
	data, err := rngid.AppendBinary(data)
	if err != nil {
		return nil, fmt.Errorf("appending EdsID: %w", err)
	}
	data = binary.BigEndian.AppendUint32(data, uint32(rngid.From))
	data = binary.BigEndian.AppendUint32(data, uint32(rngid.To))
	return data, nil
}
