package shwap

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/share"
)

// DataIDSize defines the total size of a DataID in bytes, combining the size of a RowID and the
// size of a Namespace.
const DataIDSize = RowIDSize + share.NamespaceSize

// DataID uniquely identifies a piece of namespaced data within a row of an Extended Data Square
// (EDS).
type DataID struct {
	RowID                // Embedded RowID representing the specific row in the EDS.
	DataNamespace string // DataNamespace is a string representation of the namespace to facilitate comparisons.
	// TODO(@walldiss): Remove the need for string comparisons after updating the bitswap global roots
	// cache to use reference counting.
}

// NewDataID creates a new DataID with the specified parameters. It validates the DataID against
// the provided Root before returning.
func NewDataID(height uint64, rowIdx uint16, namespace share.Namespace, root *share.Root) (DataID, error) {
	did := DataID{
		RowID: RowID{
			EdsID: EdsID{
				Height: height,
			},
			RowIndex: rowIdx,
		},
		DataNamespace: string(namespace),
	}

	if err := did.Verify(root); err != nil {
		return DataID{}, err
	}
	return did, nil
}

// Namespace retrieves the namespace part of the DataID as a share.Namespace type.
func (s DataID) Namespace() share.Namespace {
	return share.Namespace(s.DataNamespace)
}

// MarshalBinary encodes DataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (s DataID) MarshalBinary() []byte {
	data := make([]byte, 0, DataIDSize)
	return s.appendTo(data)
}

// DataIDFromBinary deserializes a DataID from its binary form. It returns an error if the binary
// data's length does not match the expected size.
func DataIDFromBinary(data []byte) (DataID, error) {
	if len(data) != DataIDSize {
		return DataID{}, fmt.Errorf("invalid DataID data length: expected %d, got %d", DataIDSize, len(data))
	}

	rid, err := RowIDFromBinary(data[:RowIDSize])
	if err != nil {
		return DataID{}, fmt.Errorf("error unmarshaling RowID: %w", err)
	}

	nsData := data[RowIDSize:]
	ns := share.Namespace(nsData)
	if err := ns.ValidateForData(); err != nil {
		return DataID{}, fmt.Errorf("error validating DataNamespace: %w", err)
	}

	return DataID{RowID: rid, DataNamespace: string(ns)}, nil
}

// Verify checks the validity of DataID's fields, including the RowID and the namespace.
func (s DataID) Verify(root *share.Root) error {
	if err := s.RowID.Verify(root); err != nil {
		return fmt.Errorf("error validating RowID: %w", err)
	}
	if err := s.Namespace().ValidateForData(); err != nil {
		return fmt.Errorf("error validating DataNamespace: %w", err)
	}

	return nil
}

// appendTo helps in appending the binary form of DataNamespace to the serialized RowID data.
func (s DataID) appendTo(data []byte) []byte {
	data = s.RowID.appendTo(data)
	return append(data, s.DataNamespace...)
}
