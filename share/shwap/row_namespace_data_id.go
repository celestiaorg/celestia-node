package shwap

import (
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

// RowNamespaceDataIDSize defines the total size of a RowNamespaceDataID in bytes, combining the
// size of a RowID and the size of a Namespace.
const RowNamespaceDataIDSize = RowIDSize + libshare.NamespaceSize

// RowNamespaceDataID uniquely identifies a piece of namespaced data within a row of an Extended
// Data Square (EDS).
type RowNamespaceDataID struct {
	RowID // Embedded RowID representing the specific row in the EDS.
	// DataNamespace is used to facilitate comparisons.
	DataNamespace libshare.Namespace
}

// NewRowNamespaceDataID creates a new RowNamespaceDataID with the specified parameters. It
// validates the RowNamespaceDataID against the provided EDS size.
func NewRowNamespaceDataID(
	height uint64,
	rowIdx int,
	namespace libshare.Namespace,
	edsSize int,
) (RowNamespaceDataID, error) {
	did := RowNamespaceDataID{
		RowID: RowID{
			EdsID: EdsID{
				height: height,
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

	ns, err := libshare.NewNamespaceFromBytes(data[RowIDSize:])
	if err != nil {
		return RowNamespaceDataID{}, fmt.Errorf("invalid namespace format: %w", err)
	}

	return RowNamespaceDataID{
		RowID:         rid,
		DataNamespace: ns,
	}, nil
}

// Equals checks equality of RowNamespaceDataID.
func (rndid *RowNamespaceDataID) Equals(other RowNamespaceDataID) bool {
	return rndid.RowID.Equals(other.RowID) && rndid.DataNamespace.Equals(other.DataNamespace)
}

// ReadFrom reads the binary form of RowNamespaceDataID from the provided reader.
func (rndid *RowNamespaceDataID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, RowNamespaceDataIDSize)
	n, err := io.ReadFull(r, data)
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
	*rndid = id
	return int64(n), nil
}

// MarshalBinary encodes RowNamespaceDataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (rndid RowNamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RowNamespaceDataIDSize)
	data, err := rndid.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes the binary form of RowNamespaceDataID to the provided writer.
func (rndid RowNamespaceDataID) WriteTo(w io.Writer) (int64, error) {
	data, err := rndid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Verify validates the RowNamespaceDataID and verifies the embedded RowID.
func (rndid RowNamespaceDataID) Verify(edsSize int) error {
	if err := rndid.RowID.Verify(edsSize); err != nil {
		return fmt.Errorf("error verifying RowID: %w", err)
	}

	return rndid.Validate()
}

// Validate performs basic field validation.
func (rndid RowNamespaceDataID) Validate() error {
	if err := rndid.RowID.Validate(); err != nil {
		return fmt.Errorf("validating RowID: %w", err)
	}
	if err := rndid.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("%w: validating DataNamespace: %w", ErrInvalidID, err)
	}

	return nil
}

// AppendBinary helps in appending the binary form of DataNamespace to the serialized RowID data.
func (rndid RowNamespaceDataID) AppendBinary(data []byte) ([]byte, error) {
	data, err := rndid.RowID.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return append(data, rndid.DataNamespace.Bytes()...), nil
}
