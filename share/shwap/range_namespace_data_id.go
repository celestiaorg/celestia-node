package shwap

import (
	"encoding/binary"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

// RangeNamespaceDataIDSize defines the size of the RangeNamespaceDataIDSize in bytes,
// combining SampleID size, Namespace size, 4 additional bytes
// for the end coordinates of share of the range and uint representation of bool flag.
const RangeNamespaceDataIDSize = EdsIDSize + libshare.NamespaceSize + 10

// RangeNamespaceDataID identifies the continuous range of shares in the DataSquare(EDS),
// starting from the given `SampleID` and contains `Length` number of shares.
type RangeNamespaceDataID struct {
	NamespaceDataID
	// coordinates from the first share of the range
	From SampleCoords
	// coordinates from the last share of the range
	To SampleCoords

	ProofsOnly bool
}

func NewRangeNamespaceDataID(
	edsID EdsID,
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	edsSize int,
	proofsOnly bool,
) (RangeNamespaceDataID, error) {
	rngid := RangeNamespaceDataID{
		NamespaceDataID: NamespaceDataID{
			EdsID:         edsID,
			DataNamespace: namespace,
		},
		From:       from,
		To:         to,
		ProofsOnly: proofsOnly,
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
	err = rngid.DataNamespace.ValidateForData()
	if err != nil {
		return err
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
	return rngid.DataNamespace.ValidateForData()
}

// ReadFrom reads the binary form of RangeNamespaceDataID from the provided reader.
func (rngid *RangeNamespaceDataID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, RangeNamespaceDataIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}
	if n != RangeNamespaceDataIDSize {
		return int64(n), fmt.Errorf("RangeNamespaceDataID: expected %d bytes, got %d", RangeNamespaceDataIDSize, n)
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
	return rngid.DataNamespace.Equals(other.DataNamespace) && rngid.From == other.From &&
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

	ns, err := libshare.NewNamespaceFromBytes(data[EdsIDSize+8 : EdsIDSize+8+libshare.NamespaceSize])
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("converting namespace from binary: %w", err)
	}

	var proofsOnly bool
	if data[RangeNamespaceDataIDSize-1] == 1 {
		proofsOnly = true
	}

	rngID := RangeNamespaceDataID{
		NamespaceDataID: NamespaceDataID{
			EdsID:         edsID,
			DataNamespace: ns,
		},
		From:       fromCoords,
		To:         toCoords,
		ProofsOnly: proofsOnly,
	}
	return rngID, rngID.Validate()
}

// MarshalBinary encodes RangeNamespaceDataID into binary form.
func (rngid RangeNamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RangeNamespaceDataIDSize)
	return rngid.appendTo(data)
}

// appendTo helps in constructing the binary representation  of RangeNamespaceDataID
// by appending all encoded fields.
func (rngid RangeNamespaceDataID) appendTo(data []byte) ([]byte, error) {
	data, err := rngid.EdsID.AppendBinary(data)
	if err != nil {
		return nil, fmt.Errorf("appending EdsID: %w", err)
	}
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.From.Row))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.From.Col))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To.Row))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To.Col))
	data = append(data, rngid.DataNamespace.Bytes()...)
	if rngid.ProofsOnly {
		data = binary.BigEndian.AppendUint16(data, 1)
	} else {
		data = binary.BigEndian.AppendUint16(data, 0)
	}
	return data, nil
}
