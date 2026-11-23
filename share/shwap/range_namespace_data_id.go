package shwap

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v4/share"
)

// RangeNamespaceDataIDSize defines the size of the RangeNamespaceDataIDSize in bytes,
// combining EdsIDSize size and 4 additional bytes
// for the start and end ODS indexes of share of the range.
const RangeNamespaceDataIDSize = EdsIDSize + 8

const rangeNamespaceDataName = "rangeNamespaceData_v0"

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
	From int
	// To specifies the index of the last share in the range(exclusively).
	To int
}

func NewRangeNamespaceDataID(
	edsID EdsID,
	from, to, odsSize int,
) (RangeNamespaceDataID, error) {
	rngid := RangeNamespaceDataID{
		EdsID: edsID,
		From:  from,
		To:    to,
	}

	err := rngid.Verify(odsSize)
	if err != nil {
		return RangeNamespaceDataID{}, fmt.Errorf("verifying range id: %w", err)
	}
	return rngid, nil
}

func (rngid RangeNamespaceDataID) Name() string {
	return rangeNamespaceDataName
}

// Verify validates the RangeNamespaceDataID fields and verifies that number of the requested shares
// does not exceed the number of shares inside the ODS.
func (rngid RangeNamespaceDataID) Verify(odsSize int) error {
	err := rngid.Validate()
	if err != nil {
		return err
	}

	sharesAmount := odsSize * odsSize
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
	err := rngid.EdsID.Validate()
	if err != nil {
		return fmt.Errorf("invalid EdsID: %w", err)
	}
	if rngid.From < 0 {
		return fmt.Errorf("%w: From: %d < 0", ErrInvalidID, rngid.From)
	}
	if rngid.To <= 0 {
		return fmt.Errorf("%w: To: %d <= 0", ErrInvalidID, rngid.To)
	}
	if rngid.From >= rngid.To {
		return fmt.Errorf("invalid range: from %d to %d", rngid.From, rngid.To)
	}
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
	if err := rngid.Validate(); err != nil {
		return int64(0), err
	}

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
		From:  int(binary.BigEndian.Uint32(data[EdsIDSize : EdsIDSize+4])),
		To:    int(binary.BigEndian.Uint32(data[EdsIDSize+4 : EdsIDSize+8])),
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
	data = binary.BigEndian.AppendUint32(data, uint32(rngid.From))
	data = binary.BigEndian.AppendUint32(data, uint32(rngid.To))
	return data, nil
}

// ResponseSize returns the memory budget for one row and its proof.
func (rngid RangeNamespaceDataID) ResponseSize(edsSize int) int {
	return rowStreamReserve(edsSize)
}

func (rngid RangeNamespaceDataID) ResponseReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	size, err := acc.Size(ctx)
	if err != nil {
		return nil, err
	}
	odsSize := size / 2
	if err := rngid.Verify(odsSize); err != nil {
		return nil, err
	}
	first, last := rngid.From/odsSize, (rngid.To-1)/odsSize
	var namespace libshare.Namespace
	return newRowStreamReader(ctx, last-first+1, func(i int) (RowNamespaceData, error) {
		row := first + i
		from, to := max(rngid.From, row*odsSize), min(rngid.To, (row+1)*odsSize)
		data, err := acc.RangeNamespaceData(ctx, from, to)
		if err != nil {
			return RowNamespaceData{}, err
		}
		shares := data.Shares[0]
		if i == 0 {
			namespace = shares[0].Namespace()
		} else if !namespace.Equals(shares[0].Namespace()) {
			return RowNamespaceData{}, fmt.Errorf("mismatched namespace in row %d", row)
		}
		return RowNamespaceData{Shares: shares, Proof: data.FirstIncompleteRowProof}, nil
	})
}
