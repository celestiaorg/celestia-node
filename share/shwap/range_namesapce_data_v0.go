package shwap

import (
	"encoding/binary"
	"fmt"
)

// RangeNamespaceDataIDSize defines the size of the RangeNamespaceDataIDV0Size in bytes,
// combining EdsIDSize size and 4 additional bytes
// for the start and end ODS indexes of share of the range.
const RangeNamespaceDataIDV0Size = EdsIDSize + 4

type RangeNamespaceDataIDV0 struct {
	RangeNamespaceDataID
}

func NewRangeNamespaceDataIDV0(
	edsID EdsID,
	from, to, odsSize int,
) (RangeNamespaceDataIDV0, error) {
	rngData, err := NewRangeNamespaceDataID(edsID, from, to, odsSize)
	if err != nil {
		return RangeNamespaceDataIDV0{}, err
	}
	return RangeNamespaceDataIDV0{RangeNamespaceDataID: rngData}, nil
}

// RangeNamespaceDataIDFromBinary deserializes a RangeNamespaceDataID from its binary form.
func RangeNamespaceDataIDV0FromBinary(data []byte) (RangeNamespaceDataIDV0, error) {
	if len(data) != RangeNamespaceDataIDSize {
		return RangeNamespaceDataIDV0{}, fmt.Errorf(
			"invalid RangeNamespaceDataID data length: expected %d, got %d", RangeNamespaceDataIDSize, len(data),
		)
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return RangeNamespaceDataIDV0{}, err
	}

	rngID := RangeNamespaceDataID{
		EdsID: edsID,
		From:  int(binary.BigEndian.Uint16(data[EdsIDSize : EdsIDSize+2])),
		To:    int(binary.BigEndian.Uint16(data[EdsIDSize+2 : EdsIDSize+4])),
	}
	return RangeNamespaceDataIDV0{RangeNamespaceDataID: rngID}, rngID.Validate()
}

// MarshalBinary encodes RangeNamespaceDataIDV0 into binary form.
func (rngid RangeNamespaceDataIDV0) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RangeNamespaceDataIDV0Size)
	return rngid.appendTo(data)
}

// appendTo helps in constructing the binary representation of RangeNamespaceDataIDV0
// by appending all encoded fields.
func (rngid RangeNamespaceDataIDV0) appendTo(data []byte) ([]byte, error) {
	data, err := rngid.AppendBinary(data)
	if err != nil {
		return nil, fmt.Errorf("appending EdsID: %w", err)
	}
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.From))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To))
	return data, nil
}

// Equals checks equality of RangeNamespaceDataIDV0.
func (rngid *RangeNamespaceDataIDV0) Equals(other RangeNamespaceDataIDV0) bool {
	return rngid.RangeNamespaceDataID.Equals(other.RangeNamespaceDataID)
}
