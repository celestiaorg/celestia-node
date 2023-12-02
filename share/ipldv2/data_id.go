package ipldv2

import (
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

// DataIDSize is the size of the DataID in bytes
// We cut 1 byte from AxisIDSize because we don't need AxisType
// as its value is always Row.
const DataIDSize = AxisIDSize - 1 + share.NamespaceSize

// DataID is an unique identifier of a namespaced Data inside EDS Axis.
type DataID struct {
	AxisID

	// DataNamespace is the namespace of the data.
	DataNamespace share.Namespace
}

// NewDataID constructs a new DataID.
func NewDataID(axisIdx int, root *share.Root, height uint64, namespace share.Namespace) DataID {
	return DataID{
		AxisID:        NewAxisID(rsmt2d.Row, uint16(axisIdx), root, height),
		DataNamespace: namespace,
	}
}

// DataIDFromCID coverts CID to DataID.
func DataIDFromCID(cid cid.Cid) (id DataID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhalling DataID: %w", err)
	}

	return id, nil
}

// Cid returns sample ID encoded as CID.
func (s *DataID) Cid() (cid.Cid, error) {
	// avoid using proto serialization for CID as it's not deterministic
	data, err := s.MarshalBinary()
	if err != nil {
		return cid.Undef, err
	}

	buf, err := mh.Encode(data, dataMultihashCode)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(dataCodec, buf), nil
}

// MarshalBinary encodes DataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (s *DataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, DataIDSize+1)
	n, err := s.AxisID.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	data = data[1:n] // cut the first byte with AxisType
	data = append(data, s.DataNamespace...)
	return data, nil
}

// UnmarshalBinary decodes DataID from binary form.
func (s *DataID) UnmarshalBinary(data []byte) error {
	if len(data) != DataIDSize {
		return fmt.Errorf("invalid data length: %d != %d", len(data), DataIDSize)
	}
	n, err := s.AxisID.UnmarshalFrom(append([]byte{byte(rsmt2d.Row)}, data...))
	if err != nil {
		return err
	}
	s.DataNamespace = data[n-1:]
	return nil
}

// Validate validates fields of DataID.
func (s *DataID) Validate() error {
	if err := s.AxisID.Validate(); err != nil {
		return fmt.Errorf("while validating AxisID: %w", err)
	}
	if err := s.DataNamespace.ValidateForData(); err != nil {
		return fmt.Errorf("while validating DataNamespace: %w", err)
	}

	return nil
}
