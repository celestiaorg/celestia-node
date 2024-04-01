package shwap

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/file"
)

// SampleIDSize is the size of the SampleID in bytes
const SampleIDSize = RowIDSize + 2

// SampleID is an unique identifier of a Sample.
type SampleID struct {
	RowID

	// ShareIndex is the index of the sampled share in the Row
	ShareIndex uint16
}

// NewSampleID constructs a new SampleID.
func NewSampleID(height uint64, smplIdx int, root *share.Root) (SampleID, error) {
	sqrLn := len(root.RowRoots)
	rowIdx, shrIdx := uint16(smplIdx/sqrLn), uint16(smplIdx%sqrLn)
	sid := SampleID{
		RowID: RowID{
			EdsID: EdsID{
				Height: height,
			},
			RowIndex: rowIdx,
		},
		ShareIndex: shrIdx,
	}

	// Store the root in the cache for verification later
	globalRootsCache.Store(sid, root)
	return sid, sid.Verify(root)
}

// SampleIDFromCID coverts CID to SampleID.
func SampleIDFromCID(cid cid.Cid) (id SampleID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	id, err = SampleIdFromBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhaling SampleID: %w", err)
	}

	return id, nil
}

// Cid returns SampleID encoded as CID.
func (sid SampleID) Cid() cid.Cid {
	// avoid using proto serialization for CID as it's not deterministic
	data := sid.MarshalBinary()

	buf, err := mh.Encode(data, sampleMultihashCode)
	if err != nil {
		panic(fmt.Errorf("encoding SampleID as CID: %w", err))
	}

	return cid.NewCidV1(sampleCodec, buf)
}

// MarshalBinary encodes SampleID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (sid SampleID) MarshalBinary() []byte {
	data := make([]byte, 0, SampleIDSize)
	return sid.appendTo(data)
}

// SampleIdFromBinary decodes SampleID from binary form.
func SampleIdFromBinary(data []byte) (SampleID, error) {
	var sid SampleID
	if len(data) != SampleIDSize {
		return sid, fmt.Errorf("invalid SampleID data length: %d != %d", len(data), SampleIDSize)
	}

	rid, err := RowIDFromBinary(data[:RowIDSize])
	if err != nil {
		return sid, fmt.Errorf("while decoding RowID: %w", err)
	}
	sid.RowID = rid
	return sid, binary.Read(bytes.NewReader(data[RowIDSize:]), binary.BigEndian, &sid.ShareIndex)
}

// Verify verifies SampleID fields.
func (sid SampleID) Verify(root *share.Root) error {
	sqrLn := len(root.ColumnRoots)
	if int(sid.ShareIndex) >= sqrLn {
		return fmt.Errorf("ShareIndex exceeds square size: %d >= %d", sid.ShareIndex, sqrLn)
	}

	return sid.RowID.Verify(root)
}

// BlockFromFile returns the IPLD block of the Sample.
func (sid SampleID) BlockFromFile(ctx context.Context, f file.EdsFile) (blocks.Block, error) {
	shr, err := f.Share(ctx, int(sid.ShareIndex), int(sid.RowID.RowIndex))
	if err != nil {
		return nil, fmt.Errorf("while getting share with proof: %w", err)
	}

	s := NewSample(sid, shr.Share, *shr.Proof, shr.Axis)
	blk, err := s.IPLDBlock()
	if err != nil {
		return nil, fmt.Errorf("while coverting to IPLD block: %w", err)
	}
	return blk, nil
}

// Release releases the verifier of the SampleID.
func (sid SampleID) Release() {
	globalRootsCache.Delete(sid)
}

func (sid SampleID) appendTo(data []byte) []byte {
	data = sid.RowID.appendTo(data)
	return binary.BigEndian.AppendUint16(data, sid.ShareIndex)
}

func (sid SampleID) key() any {
	return sid
}
