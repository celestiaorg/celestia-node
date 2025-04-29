package shwap

import (
	"bytes"
	"context"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

// NamespaceDataIDSize defines the total size of a NamespaceDataID in bytes, combining the
// size of a EdsID and the size of a Namespace.
const NamespaceDataIDSize = EdsIDSize + libshare.NamespaceSize

// NamespaceDataID filters the data in the EDS by a specific namespace.
type NamespaceDataID struct {
	// Embedding EdsID to include the block height.
	EdsID
	// DataNamespace will be used to identify the data within the EDS.
	DataNamespace libshare.Namespace
}

// NewNamespaceDataID creates a new NamespaceDataID with the specified parameters. It
// validates the namespace and returns an error if it is invalid.
func NewNamespaceDataID(height uint64, namespace libshare.Namespace) (NamespaceDataID, error) {
	ndid := NamespaceDataID{
		EdsID: EdsID{
			Height: height,
		},
		DataNamespace: namespace,
	}

	if err := ndid.Validate(); err != nil {
		return NamespaceDataID{}, err
	}
	return ndid, nil
}

func (ndid NamespaceDataID) Name() string {
	return namespaceDataName
}

// NamespaceDataIDFromBinary deserializes a NamespaceDataID from its binary form. It returns
// an error if the binary data's length does not match the expected size.
func NamespaceDataIDFromBinary(data []byte) (NamespaceDataID, error) {
	if len(data) != NamespaceDataIDSize {
		return NamespaceDataID{},
			fmt.Errorf("invalid NamespaceDataID length: expected %d, got %d", NamespaceDataIDSize, len(data))
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return NamespaceDataID{}, fmt.Errorf("error unmarshaling EDSID: %w", err)
	}

	ns, err := libshare.NewNamespaceFromBytes(data[EdsIDSize:])
	if err != nil {
		return NamespaceDataID{}, fmt.Errorf("error unmarshaling namespace: %w", err)
	}

	ndid := NamespaceDataID{
		EdsID:         edsID,
		DataNamespace: ns,
	}
	if err := ndid.Validate(); err != nil {
		return NamespaceDataID{}, err
	}
	return ndid, nil
}

// Equals checks equality of NamespaceDataID.
func (ndid *NamespaceDataID) Equals(other NamespaceDataID) bool {
	return ndid.EdsID.Equals(other.EdsID) && ndid.DataNamespace.Equals(other.DataNamespace)
}

// ReadFrom reads the binary form of NamespaceDataID from the provided reader.
func (ndid *NamespaceDataID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, NamespaceDataIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}
	if n != NamespaceDataIDSize {
		return int64(n), fmt.Errorf("NamespaceDataID: expected %d bytes, got %d", NamespaceDataIDSize, n)
	}
	id, err := NamespaceDataIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("NamespaceDataIDFromBinary: %w", err)
	}
	*ndid = id
	return int64(n), nil
}

// MarshalBinary encodes NamespaceDataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (ndid NamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, NamespaceDataIDSize)
	data, err := ndid.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes the binary form of NamespaceDataID to the provided writer.
func (ndid NamespaceDataID) WriteTo(w io.Writer) (int64, error) {
	data, err := ndid.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// Validate checks if the NamespaceDataID is valid. It checks the validity of the EdsID and the
// DataNamespace.
func (ndid NamespaceDataID) Validate() error {
	if err := ndid.EdsID.Validate(); err != nil {
		return fmt.Errorf("validating RowID: %w", err)
	}

	if err := ndid.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("%w: validating DataNamespace: %w", ErrInvalidID, err)
	}
	return nil
}

// AppendBinary helps in appending the binary form of DataNamespace to the serialized RowID data.
func (ndid NamespaceDataID) AppendBinary(data []byte) ([]byte, error) {
	data, err := ndid.EdsID.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	return append(data, ndid.DataNamespace.Bytes()...), nil
}

func (ndid NamespaceDataID) FetchContainerReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	rows, err := EDSData(ctx, acc, ndid.DataNamespace)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	_, err = rows.WriteTo(buf)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
