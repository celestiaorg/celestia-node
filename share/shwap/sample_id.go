package shwap

import (
	"context"
	"encoding/binary"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/file"
)

//TODO(@walldiss): maybe move into separate subpkg?

// SampleIDSize is the size of the SampleID in bytes
const SampleIDSize = RowIDSize + 2

// SampleID is an unique identifier of a Sample.
type SampleID struct {
	// TODO(@walldiss): why embed instead of just having a field?
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
			RowIndex: rowIdx,
			Height:   height,
		},
		ShareIndex: shrIdx,
	}
	return sid, sid.Verify(root)
}

// SampleIDFromCID coverts CID to SampleID.
func SampleIDFromCID(cid cid.Cid) (id SampleID, err error) {
	if err = validateCID(cid); err != nil {
		return id, err
	}

	err = id.UnmarshalBinary(cid.Hash()[mhPrefixSize:])
	if err != nil {
		return id, fmt.Errorf("while unmarhaling SampleID: %w", err)
	}

	return id, nil
}

// Cid returns SampleID encoded as CID.
func (sid SampleID) Cid() cid.Cid {
	// avoid using proto serialization for CID as it's not deterministic
	data, err := sid.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling SampleID: %w", err))
	}

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
func (sid SampleID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, SampleIDSize)
	n, err := sid.RowID.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	data = data[:n]
	data = binary.LittleEndian.AppendUint16(data, sid.ShareIndex)
	return data, nil
}

// UnmarshalBinary decodes SampleID from binary form.
func (sid *SampleID) UnmarshalBinary(data []byte) error {
	if len(data) != SampleIDSize {
		return fmt.Errorf("invalid SampleID data length: %d != %d", len(data), SampleIDSize)
	}
	n, err := sid.RowID.UnmarshalFrom(data)
	if err != nil {
		return err
	}
	data = data[n:]
	sid.ShareIndex = binary.LittleEndian.Uint16(data)
	return nil
}

// Verify verifies SampleID fields.
func (sid SampleID) Verify(root *share.Root) error {
	sqrLn := len(root.ColumnRoots)
	if int(sid.ShareIndex) >= sqrLn {
		return fmt.Errorf("ShareIndex exceeds square size: %d >= %d", sid.ShareIndex, sqrLn)
	}

	return sid.RowID.Verify(root)
}

func (sid SampleID) GetHeight() uint64 {
	return sid.RowID.Height
}

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
