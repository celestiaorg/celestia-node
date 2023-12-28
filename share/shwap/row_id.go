package shwap

import (
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/celestia-node/share"
)

// TODO:
// * Remove RowHash
// 	* Change validation
// * Remove IDs from responses

// RowIDSize is the size of the RowID in bytes
const RowIDSize = 10

// RowID is an unique identifier of a Row.
type RowID struct {
	// Height of the block.
	// Needed to identify block's data square in the whole chain
	Height uint64
	// RowIndex is the index of the axis(row, col) in the data square
	RowIndex uint16
}

// NewRowID constructs a new RowID.
func NewRowID(height uint64, rowIdx uint16, root *share.Root) (RowID, error) {
	rid := RowID{
		RowIndex: rowIdx,
		Height:   height,
	}
	return rid, rid.Verify(root)
}

// RowIDFromCID coverts CID to RowID.
func RowIDFromCID(cid cid.Cid) (id RowID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhaling RowID: %w", err)
	}

	return id, nil
}

// Cid returns RowID encoded as CID.
func (rid RowID) Cid() cid.Cid {
	data, err := rid.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling RowID: %w", err))
	}

	buf, err := mh.Encode(data, rowMultihashCode)
	if err != nil {
		panic(fmt.Errorf("encoding RowID as CID: %w", err))
	}

	return cid.NewCidV1(rowCodec, buf)
}

// MarshalTo encodes RowID into given byte slice.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (rid RowID) MarshalTo(data []byte) (int, error) {
	data = binary.LittleEndian.AppendUint64(data, rid.Height)
	data = binary.LittleEndian.AppendUint16(data, rid.RowIndex)
	return RowIDSize, nil
}

// UnmarshalFrom decodes RowID from given byte slice.
func (rid *RowID) UnmarshalFrom(data []byte) (int, error) {
	rid.Height = binary.LittleEndian.Uint64(data[2:])
	rid.RowIndex = binary.LittleEndian.Uint16(data)
	return RowIDSize, nil
}

// MarshalBinary encodes RowID into binary form.
func (rid RowID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, RowIDSize)
	n, err := rid.MarshalTo(data)
	return data[:n], err
}

// UnmarshalBinary decodes RowID from binary form.
func (rid *RowID) UnmarshalBinary(data []byte) error {
	if len(data) != RowIDSize {
		return fmt.Errorf("invalid RowID data length: %d != %d", len(data), RowIDSize)
	}
	_, err := rid.UnmarshalFrom(data)
	return err
}

// Verify verifies RowID fields.
func (rid RowID) Verify(root *share.Root) error {
	if root == nil {
		return fmt.Errorf("nil Root")
	}
	if rid.Height == 0 {
		return fmt.Errorf("zero Height")
	}

	sqrLn := len(root.RowRoots)
	if int(rid.RowIndex) >= sqrLn {
		return fmt.Errorf("RowIndex exceeds square size: %d >= %d", rid.RowIndex, sqrLn)
	}

	return nil
}
