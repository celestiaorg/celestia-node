package shwap

import (
	"fmt"
	"io"

	"github.com/celestiaorg/celestia-node/share"
)

// RowNamespaceDataIDSize defines the total size of a RowNamespaceDataID in bytes, combining the
// size of a RowID and the size of a Namespace.
const RowNamespaceDataIDSize = RowIDSize + share.NamespaceSize

// RowNamespaceDataID uniquely identifies a piece of namespaced data within a row of an Extended
// Data Square (EDS).
type RowNamespaceDataID struct {
	RowID                         // Embedded RowID representing the specific row in the EDS.
	DataNamespace share.Namespace // DataNamespace is a string representation of the namespace to facilitate comparisons.
}

// NewRowNamespaceDataID creates a new RowNamespaceDataID with the specified parameters. It
// validates the RowNamespaceDataID against the provided EDS size.
func NewRowNamespaceDataID(
	height uint64,
	rowIdx int,
	namespace share.Namespace,
	edsSize int,
) (RowNamespaceDataID, error) {
	did := RowNamespaceDataID{
		RowID: RowID{
			EdsID: EdsID{
				Height: height,
			},
			RowIndex: rowIdx,
		},
		DataNamespace: namespace,
	}

	if err := did.Verify(edsSize); err != nil {
		return RowNamespaceDataID{}, fmt.Errorf("verifying RowNamespaceDataID: %w", err)
	}
	return did, nil
}

// RowNamespaceDataIDFromBinary deserializes a RowNamespaceDataID from its binary form. It returns
// an error if the binary data's length does not match the expected size.
func RowNamespaceDataIDFromBinary(data []byte) (RowNamespaceDataID, error) {
	if len(data) != RowNamespaceDataIDSize {
		return RowNamespaceDataID{},
			fmt.Errorf("invalid RowNamespaceDataID length: expected %d, got %d", RowNamespaceDataIDSize, len(data))
	}

	rid, err := RowIDFromBinary(data[:RowIDSize])
	if err != nil {
		return RowNamespaceDataID{}, fmt.Errorf("unmarshaling RowID: %w", err)
	}

	rndid := RowNamespaceDataID{
		RowID:         rid,
		DataNamespace: data[RowIDSize:],
	}
	if err := rndid.Validate(); err != nil {
		return RowNamespaceDataID{}, fmt.Errorf("validating RowNamespaceDataID: %w", err)
	}

	return rndid, nil
}

// ReadFrom reads the binary form of RowNamespaceDataID from the provided reader.
func (s *RowNamespaceDataID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, RowNamespaceDataIDSize)
	n, err := r.Read(data)
	if err != nil {
		return int64(n), err
	}
	if n != RowNamespaceDataIDSize {
		return int64(n), fmt.Errorf("RowNamespaceDataID: expected %d bytes, got %d", RowNamespaceDataIDSize, n)
	}
	id, err := RowNamespaceDataIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("RowNamespaceDataIDFromBinary: %w", err)
	}
	*s = id
	return int64(n), nil
}

// MarshalBinary encodes RowNamespaceDataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (s RowNamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RowNamespaceDataIDSize)
	return s.appendTo(data), nil
}

// WriteTo writes the binary form of RowNamespaceDataID to the provided writer.
func (s RowNamespaceDataID) WriteTo(w io.Writer) (int64, error) {
	data, err := s.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Verify validates the RowNamespaceDataID and verifies the embedded RowID.
func (s RowNamespaceDataID) Verify(edsSize int) error {
	if err := s.RowID.Verify(edsSize); err != nil {
		return fmt.Errorf("error verifying RowID: %w", err)
	}

	return s.Validate()
}

// Validate performs basic field validation.
func (s RowNamespaceDataID) Validate() error {
	if err := s.RowID.Validate(); err != nil {
		return fmt.Errorf("validating RowID: %w", err)
	}
	if err := s.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("%w: validating DataNamespace: %w", ErrInvalidID, err)
	}

	return nil
}

// appendTo helps in appending the binary form of DataNamespace to the serialized RowID data.
func (s RowNamespaceDataID) appendTo(data []byte) []byte {
	data = s.RowID.appendTo(data)
	return append(data, s.DataNamespace...)
}
