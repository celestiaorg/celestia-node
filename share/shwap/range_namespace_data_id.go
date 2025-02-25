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
const RangeNamespaceDataIDSize = SampleIDSize + libshare.NamespaceSize + 6

// RangeNamespaceDataID identifies the continuous range of shares in the DataSquare(EDS),
// starting from the given `SampleID` and contains `Length` number of shares.
type RangeNamespaceDataID struct {
	SampleID
	//  DataNamespace is a string representation of the namespace of the requested range.
	DataNamespace libshare.Namespace
	// coordinates from the last share of the range
	To SampleCoords

	ProofsOnly bool
}

func NewRangeNamespaceDataID(
	height uint64,
	namespace libshare.Namespace,
	from SampleCoords,
	to SampleCoords,
	edsSize int,
	proofsOnly bool,
) (RangeNamespaceDataID, error) {
	sampleID, err := NewSampleID(height, from, edsSize)
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("creating sample ID: %w", err)
	}

	rngid := RangeNamespaceDataID{
		SampleID:      sampleID,
		DataNamespace: namespace,
		To:            to,
		ProofsOnly:    proofsOnly,
	}

	err = rngid.Verify(edsSize)
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("verifying range id: %w", err)
	}
	return rngid, nil
}

// Verify validates the RangeNamespaceDataID fields and verifies that number of the requested shares
// does not exceed the number of shares inside the ODS.
func (rngid RangeNamespaceDataID) Verify(edsSize int) error {
	err := rngid.DataNamespace.ValidateForData()
	if err != nil {
		return err
	}
	if err := rngid.SampleID.Verify(edsSize); err != nil {
		return fmt.Errorf("RangeNamespaceDataID: sample id verification: %w", err)
	}

	// verify that to is not exceed that edsSize
	_, err = SampleCoordsAs1DIndex(rngid.To, edsSize)
	if err != nil {
		return err
	}
	return nil
}

// Validate performs basic fields validation.
func (rngid RangeNamespaceDataID) Validate() error {
	if err := rngid.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("RangeNamespaceDataID: namespace validation: %w", err)
	}
	return rngid.SampleID.Validate()
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
	return rngid.SampleID.Equals(other.SampleID) && rngid.DataNamespace.Equals(other.DataNamespace) &&
		rngid.To == other.To
}

// RangeNamespaceDataIDFromBinary deserializes a RangeNamespaceDataID from its binary form.
func RangeNamespaceDataIDFromBinary(data []byte) (RangeNamespaceDataID, error) {
	if len(data) != RangeNamespaceDataIDSize {
		return RangeNamespaceDataID{}, fmt.Errorf(
			"invalid RangeNamespaceDataID data length: expected %d, got %d", RangeNamespaceDataIDSize, len(data),
		)
	}

	sid, err := SampleIDFromBinary(data[:SampleIDSize])
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("converting SampleId from binary: %w", err)
	}

	ns, err := libshare.NewNamespaceFromBytes(data[SampleIDSize : libshare.NamespaceSize+SampleIDSize])
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("converting namespace from binary: %w", err)
	}

	toCoords := SampleCoords{
		Row: int(binary.BigEndian.Uint16(data[libshare.NamespaceSize+SampleIDSize : libshare.NamespaceSize+SampleIDSize+2])),
		Col: int(binary.BigEndian.Uint16(data[libshare.NamespaceSize+SampleIDSize+2 : RangeNamespaceDataIDSize-2])),
	}

	var proofsOnly bool
	if data[RangeNamespaceDataIDSize-1] == 1 {
		proofsOnly = true
	}

	rngID := RangeNamespaceDataID{
		SampleID:      sid,
		DataNamespace: ns,
		To:            toCoords,
		ProofsOnly:    proofsOnly,
	}
	return rngID, rngID.Validate()
}

// MarshalBinary encodes RangeNamespaceDataID into binary form.
func (rngid RangeNamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RangeNamespaceDataIDSize)
	return rngid.appendTo(data), nil
}

// appendTo helps in constructing the binary representation  of RangeNamespaceDataID
// by appending all encoded fields.
func (rngid RangeNamespaceDataID) appendTo(data []byte) []byte {
	data = rngid.SampleID.appendTo(data)
	data = append(data, rngid.DataNamespace.Bytes()...)
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To.Row))
	data = binary.BigEndian.AppendUint16(data, uint16(rngid.To.Col))

	if rngid.ProofsOnly {
		data = binary.BigEndian.AppendUint16(data, 1)
	} else {
		data = binary.BigEndian.AppendUint16(data, 0)
	}
	return data
}
