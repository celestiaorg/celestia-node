package shwap

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// NamespaceDataIDSize defines the total size of a RowNamespaceDataID in bytes, combining the
// size of a RowID and the size of a Namespace.
const NamespaceDataIDSize = EdsIDSize + 4 + share.NamespaceSize

// RowNamespaceDataID uniquely identifies a piece of namespaced data within a row of an Extended
// Data Square (EDS).
type NamespaceDataID struct {
	// Embedding EdsID to include the block height in RowID.
	EdsID
	// DataNamespace is a string representation of the namespace to facilitate comparisons.
	DataNamespace share.Namespace
}

// NewNamespaceDataID creates a new RowNamespaceDataID with the specified parameters. It
// validates the RowNamespaceDataID against the provided Root before returning.
func NewNamespaceDataID(
	height uint64,
	namespace share.Namespace,
) (NamespaceDataID, error) {
	ndid := NamespaceDataID{
		EdsID: EdsID{
			Height: height,
		},
		DataNamespace: namespace,
	}

	if err := ndid.Verify(); err != nil {
		return NamespaceDataID{}, err
	}
	return ndid, nil
}

// NamespaceDataIDFromBinary deserializes a RowNamespaceDataID from its binary form. It returns
// an error if the binary data's length does not match the expected size.
func NamespaceDataIDFromBinary(data []byte) (NamespaceDataID, error) {
	if len(data) != NamespaceDataIDSize {
		return NamespaceDataID{},
			fmt.Errorf("invalid RowNamespaceDataID length: expected %d, got %d", RowNamespaceDataIDSize, len(data))
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return NamespaceDataID{}, fmt.Errorf("error unmarshaling RowID: %w", err)
	}

	ns := share.Namespace(data[EdsIDSize:])
	if err := ns.ValidateForData(); err != nil {
		return NamespaceDataID{}, fmt.Errorf("error validating DataNamespace: %w", err)
	}

	return NamespaceDataID{
		EdsID:         edsID,
		DataNamespace: ns,
	}, nil
}

// MarshalBinary encodes RowNamespaceDataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (ndid NamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, NamespaceDataIDSize)
	return ndid.appendTo(data), nil
}

// Verify checks the validity of RowNamespaceDataID's fields, including the RowID and the
// namespace.
func (ndid NamespaceDataID) Verify() error {
	return ndid.Validate()
}

func (ndid NamespaceDataID) Validate() error {
	if err := ndid.EdsID.Validate(); err != nil {
		return fmt.Errorf("error validating RowID: %w", err)
	}
	if err := ndid.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("%w: error validating DataNamespace: %w", ErrInvalidShwapID, err)
	}
	return nil
}

// appendTo helps in appending the binary form of DataNamespace to the serialized RowID data.
func (ndid NamespaceDataID) appendTo(data []byte) []byte {
	data = ndid.EdsID.appendTo(data)
	return append(data, ndid.DataNamespace...)
}
