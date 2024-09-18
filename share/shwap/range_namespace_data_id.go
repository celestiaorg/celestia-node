package shwap

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
	"io"
)

// RangeNamespaceDataIDSize defines the size of the RangeNamespaceDataIDSize in bytes, combining SampleID size, Namespace size,
// 2 additional bytes for length of the range and a uint representation of bool flag.
const RangeNamespaceDataIDSize = SampleIDSize + share.NamespaceSize + 2 + 2

// RangeNamespaceDataID identifies the continuous range of shares in the DataSquare(EDS),
// starting from the given `SampleID` and contains `Length` number of shares.
type RangeNamespaceDataID struct {
	SampleID
	//  RangeNamespace is a string representation of the namespace of the requested range.
	RangeNamespace share.Namespace
	// Length is a number of shares to of the requested range.
	Length uint16
	// ProofsOnly specifies whether user expects to get Proofs only.
	ProofsOnly bool
}

func NewRangeNamespaceDataID(
	sampleID SampleID,
	namespace share.Namespace,
	length uint16,
	proofsOnly bool,
	edsSize int,
) (RangeNamespaceDataID, error) {
	rngid := RangeNamespaceDataID{
		SampleID:       sampleID,
		RangeNamespace: namespace,
		Length:         length,
		ProofsOnly:     proofsOnly,
	}

	err := rngid.Verify(edsSize)
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("verifying range id: %w", err)
	}
	return rngid, nil
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

	return RangeNamespaceDataID{
		SampleID:       sid,
		RangeNamespace: data[SampleIDSize : SampleIDSize+share.NamespaceSize],
		Length:         binary.BigEndian.Uint16(data[SampleIDSize+share.NamespaceSize : SampleIDSize+share.NamespaceSize+2]),
		ProofsOnly:     data[len(data)-1] == 1,
	}, nil
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

// MarshalBinary encodes RangeNamespaceDataID into binary form.
func (rngid RangeNamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RangeNamespaceDataIDSize)
	return rngid.appendTo(data), nil
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

// Verify validates the RangeNamespaceDataID fields and verifies that number of the requested shares
// does not exceed the number of shares inside the ODS.
func (rngid RangeNamespaceDataID) Verify(edsSize int) error {
	if err := rngid.SampleID.Verify(edsSize); err != nil {
		return fmt.Errorf("RangeNamespaceDataID: sample id verification: %w", err)
	}

	endIndex := uint16(rngid.SampleID.ShareIndex) + rngid.Length

	// check that end index is not bigger than amount of shares in the ODS.
	if endIndex > uint16(edsSize) {
		return errors.New("RangeNamespaceDataID: end index exceeds ODS length")
	}
	return nil
}

// Validate performs basic fields validation.
func (rngid RangeNamespaceDataID) Validate() error {
	if err := rngid.RangeNamespace.Validate(); err != nil {
		return fmt.Errorf("RangeNamespaceDataID: namespace validation: %w", err)
	}
	return rngid.SampleID.Validate()
}

// appendTo helps in constructing the binary representation  of RangeNamespaceDataID
// by appending all encoded fields.
func (rngid RangeNamespaceDataID) appendTo(data []byte) []byte {
	data = rngid.SampleID.appendTo(data)
	data = append(data, rngid.RangeNamespace...)
	data = binary.BigEndian.AppendUint16(data, rngid.Length)

	if rngid.ProofsOnly {
		data = binary.BigEndian.AppendUint16(data, 1)
	} else {
		data = binary.BigEndian.AppendUint16(data, 0)
	}
	return data
}
