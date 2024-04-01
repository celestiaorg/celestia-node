package shwap

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/file"
)

// RowIDSize is the size of the RowID in bytes
const RowIDSize = EdsIDSize + 2

// RowID is an unique identifier of a Row.
type RowID struct {
	EdsID

	// RowIndex is the index of the axis(row, col) in the data square
	RowIndex uint16
}

// NewRowID constructs a new RowID.
func NewRowID(height uint64, rowIdx uint16, root *share.Root) (RowID, error) {
	rid := RowID{
		EdsID: EdsID{
			Height: height,
		},
		RowIndex: rowIdx,
	}

	// Store the root in the cache for verification later
	globalRootsCache.Store(rid, root)
	return rid, rid.Verify(root)
}

// RowIDFromCID coverts CID to RowID.
func RowIDFromCID(cid cid.Cid) (id RowID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	rid, err := RowIDFromBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhaling RowID: %w", err)
	}
	return rid, nil
}

// Cid returns RowID encoded as CID.
func (rid RowID) Cid() cid.Cid {
	data := rid.MarshalBinary()

	buf, err := mh.Encode(data, rowMultihashCode)
	if err != nil {
		panic(fmt.Errorf("encoding RowID as CID: %w", err))
	}

	return cid.NewCidV1(rowCodec, buf)
}

// MarshalBinary encodes RowID into binary form.
func (rid RowID) MarshalBinary() []byte {
	data := make([]byte, 0, RowIDSize)
	return rid.appendTo(data)
}

// RowIDFromBinary decodes RowID from binary form.
func RowIDFromBinary(data []byte) (RowID, error) {
	var rid RowID
	if len(data) != RowIDSize {
		return rid, fmt.Errorf("invalid RowID data length: %d != %d", len(data), RowIDSize)
	}
	eid, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return rid, fmt.Errorf("while decoding EdsID: %w", err)
	}
	rid.EdsID = eid
	return rid, binary.Read(bytes.NewReader(data[EdsIDSize:]), binary.BigEndian, &rid.RowIndex)
}

// Verify verifies RowID fields.
func (rid RowID) Verify(root *share.Root) error {
	if err := rid.EdsID.Verify(root); err != nil {
		return err
	}

	sqrLn := len(root.RowRoots)
	if int(rid.RowIndex) >= sqrLn {
		return fmt.Errorf("RowIndex exceeds square size: %d >= %d", rid.RowIndex, sqrLn)
	}

	return nil
}

// BlockFromFile returns the IPLD block of the RowID from the given file.
func (rid RowID) BlockFromFile(ctx context.Context, f file.EdsFile) (blocks.Block, error) {
	axisHalf, err := f.AxisHalf(ctx, rsmt2d.Row, int(rid.RowIndex))
	if err != nil {
		return nil, fmt.Errorf("while getting AxisHalf: %w", err)
	}

	shares := axisHalf.Shares
	// If it's a parity axis, we need to get the left half of the shares
	if axisHalf.IsParity {
		axis, err := axisHalf.Extended()
		if err != nil {
			return nil, fmt.Errorf("while getting extended shares: %w", err)
		}
		shares = axis[:len(axis)/2]
	}

	s := NewRow(rid, shares)
	blk, err := s.IPLDBlock()
	if err != nil {
		return nil, fmt.Errorf("while coverting to IPLD block: %w", err)
	}
	return blk, nil
}

// Release releases the verifier of the RowID.
func (rid RowID) Release() {
	globalRootsCache.Delete(rid)
}

func (rid RowID) appendTo(data []byte) []byte {
	data = rid.EdsID.appendTo(data)
	return binary.BigEndian.AppendUint16(data, rid.RowIndex)
}

func (rid RowID) key() any {
	return rid
}
