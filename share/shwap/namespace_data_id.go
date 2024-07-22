package shwap

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// NamespaceDataIDSize defines the total size of a NamespaceDataID in bytes, combining the
// size of a EdsID and the size of a Namespace.
const NamespaceDataIDSize = EdsIDSize + share.NamespaceSize

// NamespaceDataID filters the data in the EDS by a specific namespace.
type NamespaceDataID struct {
	// Embedding EdsID to include the block height.
	EdsID
	// DataNamespace will be used to identify the data within the EDS.
	DataNamespace share.Namespace
}

// NewNamespaceDataID creates a new NamespaceDataID with the specified parameters. It
// validates the namespace and returns an error if it is invalid.
func NewNamespaceDataID(height uint64, namespace share.Namespace) (NamespaceDataID, error) {
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

// NamespaceDataIDFromBinary deserializes a NamespaceDataID from its binary form. It returns
// an error if the binary data's length does not match the expected size.
func NamespaceDataIDFromBinary(data []byte) (NamespaceDataID, error) {
	if len(data) != NamespaceDataIDSize {
		return NamespaceDataID{},
			fmt.Errorf("invalid NamespaceDataID length: expected %d, got %d", NamespaceDataIDSize, len(data))
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return NamespaceDataID{}, fmt.Errorf("error unmarshaling RowID: %w", err)
	}

	ns := share.Namespace(data[EdsIDSize:])
	ndid := NamespaceDataID{
		EdsID:         edsID,
		DataNamespace: ns,
	}
	if err := ndid.Validate(); err != nil {
		return NamespaceDataID{}, err
	}
	return ndid, nil
}

// MarshalBinary encodes NamespaceDataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (ndid NamespaceDataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, NamespaceDataIDSize)
	return ndid.appendTo(data), nil
}

// Validate checks if the NamespaceDataID is valid. It checks the validity of the EdsID and the
// DataNamespace.
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
