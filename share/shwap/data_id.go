package shwap

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/file"
)

// DataIDSize is the size of the DataID in bytes.
const DataIDSize = RowIDSize + share.NamespaceSize

// DataID is an unique identifier of a namespaced Data inside EDS Row.
type DataID struct {
	// TODO(@walldiss): why embed instead of just having a field?
	RowID

	// DataNamespace is the namespace of the data
	// It's string formatted to keep DataID comparable
	DataNamespace string
}

// NewDataID constructs a new DataID.
func NewDataID(height uint64, rowIdx uint16, namespace share.Namespace, root *share.Root) (DataID, error) {
	did := DataID{
		RowID: RowID{
			RowIndex: rowIdx,
			Height:   height,
		},
		DataNamespace: string(namespace),
	}
	return did, did.Verify(root)
}

// DataIDFromCID coverts CID to DataID.
func DataIDFromCID(cid cid.Cid) (id DataID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("unmarhalling DataID: %w", err)
	}

	return id, nil
}

// Namespace returns the namespace of the DataID.
func (s DataID) Namespace() share.Namespace {
	return share.Namespace(s.DataNamespace)
}

// Cid returns DataID encoded as CID.
func (s DataID) Cid() cid.Cid {
	// avoid using proto serialization for CID as it's not deterministic
	data, err := s.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling DataID: %w", err))
	}

	buf, err := mh.Encode(data, dataMultihashCode)
	if err != nil {
		panic(fmt.Errorf("encoding DataID as CID: %w", err))
	}

	return cid.NewCidV1(dataCodec, buf)
}

// MarshalBinary encodes DataID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (s DataID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, DataIDSize)
	n, err := s.RowID.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	data = data[:n]
	data = append(data, s.DataNamespace...)
	return data, nil
}

// UnmarshalBinary decodes DataID from binary form.
func (s *DataID) UnmarshalBinary(data []byte) error {
	if len(data) != DataIDSize {
		return fmt.Errorf("invalid DataID data length: %d != %d", len(data), DataIDSize)
	}
	n, err := s.RowID.UnmarshalFrom(data)
	if err != nil {
		return err
	}

	ns := share.Namespace(data[n:])
	if err = ns.ValidateForData(); err != nil {
		return err
	}

	s.DataNamespace = string(ns)
	return nil
}

// Verify verifies DataID fields.
func (s DataID) Verify(root *share.Root) error {
	if err := s.RowID.Verify(root); err != nil {
		return fmt.Errorf("validating RowID: %w", err)
	}
	if err := s.Namespace().ValidateForData(); err != nil {
		return fmt.Errorf("validating DataNamespace: %w", err)
	}

	return nil
}

func (s DataID) GetHeight() uint64 {
	return s.RowID.GetHeight()
}

func (s DataID) BlockFromFile(ctx context.Context, f file.EdsFile) (blocks.Block, error) {
	data, err := f.Data(ctx, s.Namespace(), int(s.RowIndex))
	if err != nil {
		return nil, fmt.Errorf("while getting Data: %w", err)
	}

	d := NewData(s, data.Shares, *data.Proof)
	blk, err := d.IPLDBlock()
	if err != nil {
		return nil, fmt.Errorf("while coverting Data to IPLD block: %w", err)
	}

	err = f.Close()
	if err != nil {
		return nil, fmt.Errorf("while closing ODS file: %w", err)
	}

	return blk, nil
}
