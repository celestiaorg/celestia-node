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
	RowID

	// DataNamespace is the namespace of the data
	// It's string formatted to keep DataID comparable
	DataNamespace string
}

// NewDataID constructs a new DataID.
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

	// Store the root in the cache for verification later
	globalRootsCache.Store(did, root)
	return did, did.Verify(root)
}

// DataIDFromCID coverts CID to DataID.
func DataIDFromCID(cid cid.Cid) (id DataID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	id, err = DataIDFromBinary(cid.Hash()[mhPrefixSize:])
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
	data := s.MarshalBinary()

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
func (s DataID) MarshalBinary() []byte {
	data := make([]byte, 0, DataIDSize)
	return s.appendTo(data)
}

// DataIDFromBinary decodes DataID from binary form.
func DataIDFromBinary(data []byte) (DataID, error) {
	var did DataID
	if len(data) != DataIDSize {
		return did, fmt.Errorf("invalid DataID data length: %d != %d", len(data), DataIDSize)
	}
	rid, err := RowIDFromBinary(data[:RowIDSize])
	if err != nil {
		return did, fmt.Errorf("while unmarhaling RowID: %w", err)
	}
	did.RowID = rid
	ns := share.Namespace(data[RowIDSize:])
	if err = ns.ValidateForData(); err != nil {
		return did, fmt.Errorf("validating DataNamespace: %w", err)
	}
	did.DataNamespace = string(ns)
	return did, err
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

// BlockFromFile returns the IPLD block of the DataID from the given file.
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
	return blk, nil
}

// Release releases the verifier of the DataID.
func (s DataID) Release() {
	globalRootsCache.Delete(s)
}

func (s DataID) appendTo(data []byte) []byte {
	data = s.RowID.appendTo(data)
	return append(data, s.DataNamespace...)
}

func (s DataID) key() any {
	return s
}
