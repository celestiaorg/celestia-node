package shwap

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/celestiaorg/nmt"
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

// ResponseSize returns the memory held while streaming the response, not its wire size.
// See rowStreamReserve.
func (rngid RangeNamespaceDataID) ResponseSize(edsSize int) int {
	return rowStreamReserve(edsSize)
}

// ResponseReader streams the range row by row, in the same wire format
// RangeNamespaceData.WriteTo produces: one RowNamespaceData per row, carrying a proof only for an
// incomplete first or last row. Accessor caching is turned off for the stream, see
// NamespaceDataID.ResponseReader.
func (rngid RangeNamespaceDataID) ResponseReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	edsSize, err := acc.Size(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting EDS size: %w", err)
	}
	odsSize := edsSize / 2

	from, err := SampleCoordsFrom1DIndex(rngid.From, odsSize)
	if err != nil {
		return nil, fmt.Errorf("getting range start coordinates: %w", err)
	}
	// To is exclusive, so the last share of the range sits at To-1.
	to, err := SampleCoordsFrom1DIndex(rngid.To-1, odsSize)
	if err != nil {
		return nil, fmt.Errorf("getting range end coordinates: %w", err)
	}

	rows := to.Row - from.Row + 1
	multiRow := rows > 1
	// Same rule as RangeNamespaceDataFromShares: a row is proven only where the range cuts it.
	startProof := from.Col != 0 || (!multiRow && to.Col != odsSize-1)
	endProof := multiRow && to.Col != odsSize-1

	ctx = WithCacheDisabled(ctx)
	// The first row is read up front: it fixes the namespace every share in the range must carry,
	// which the concurrent fetchers below need before they can check their own rows.
	firstRow, err := extendedRow(ctx, acc, from.Row)
	if err != nil {
		return nil, err
	}
	namespace := firstRow[from.Col].Namespace()

	r, err := newRowStreamReader(ctx, rows, func(ctx context.Context, row int) ([]byte, error) {
		shares := firstRow
		if row > 0 {
			var err error
			shares, err = extendedRow(ctx, acc, from.Row+row)
			if err != nil {
				return nil, err
			}
		}

		start, end := 0, odsSize
		if row == 0 {
			start = from.Col
			if !multiRow {
				end = to.Col + 1
			}
		} else if row == rows-1 {
			end = to.Col + 1
		}

		var proof *nmt.Proof
		if (row == 0 && startProof) || (row == rows-1 && endProof) {
			generated, err := GenerateSharesProofs(from.Row+row, start, end, odsSize, shares)
			if err != nil {
				return nil, fmt.Errorf("generating proof for row %d: %w", from.Row+row, err)
			}
			proof = generated
		}

		selected := shares[start:end]
		for col, shr := range selected {
			if !namespace.Equals(shr.Namespace()) {
				return nil, fmt.Errorf("mismatched namespace for share at: row %d, col: %d", row, col)
			}
		}

		var buf bytes.Buffer
		rowData := RowNamespaceData{Shares: selected, Proof: proof}
		if _, err := rowData.WriteTo(&buf); err != nil {
			return nil, fmt.Errorf("writing row %d: %w", from.Row+row, err)
		}
		return buf.Bytes(), nil
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}
