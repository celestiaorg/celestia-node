package bitswap

import (
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	bitswappb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"
)

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7810

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7811
)

func init() {
	RegisterBlock(
		sampleMultihashCode,
		sampleCodec,
		shwap.SampleIDSize,
		func(cid cid.Cid) (blockBuilder, error) {
			return EmptySampleBlockFromCID(cid)
		},
	)
}

type SampleBlock struct {
	ID        shwap.SampleID
	Container *shwap.Sample
}

func NewEmptySampleBlock(height uint64, rowIdx, colIdx int, root *share.Root) (*SampleBlock, error) {
	id, err := shwap.NewSampleID(height, rowIdx, colIdx, root)
	if err != nil {
		return nil, err
	}

	return &SampleBlock{ID: id}, nil
}

// EmptySampleBlockFromCID coverts CID to SampleBlock.
func EmptySampleBlockFromCID(cid cid.Cid) (*SampleBlock, error) {
	sidData, err := extractCID(cid)
	if err != nil {
		return nil, err
	}

	sid, err := shwap.SampleIDFromBinary(sidData)
	if err != nil {
		return nil, fmt.Errorf("while unmarhaling SampleBlock: %w", err)
	}

	return &SampleBlock{ID: sid}, nil
}

func (sb *SampleBlock) IsEmpty() bool {
	return sb.Container == nil
}

func (sb *SampleBlock) String() string {
	data, err := sb.ID.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling SampleBlock: %w", err))
	}
	return string(data)
}

func (sb *SampleBlock) CID() cid.Cid {
	return encodeCID(sb.ID, sampleMultihashCode, sampleCodec)
}

func (sb *SampleBlock) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	smpl, err := shwap.SampleFromEDS(eds, rsmt2d.Row, sb.ID.RowIndex, sb.ID.ShareIndex)
	if err != nil {
		return nil, err
	}

	smplID, err := sb.ID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling SampleBlock: %w", err)
	}

	smplBlk := bitswappb.SampleBlock{
		SampleId: smplID,
		Sample:   smpl.ToProto(),
	}

	blkData, err := smplBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling SampleBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(blkData, sb.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling block: %w", err)
	}

	return blk, nil
}

func (sb *SampleBlock) Populate(root *share.Root) PopulateFn {
	return func(data []byte) error {
		var sampleBlk bitswappb.SampleBlock
		if err := sampleBlk.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling SampleBlock: %w", err)
		}

		cntr := shwap.SampleFromProto(sampleBlk.Sample)
		if err := cntr.Validate(root, sb.ID.RowIndex, sb.ID.ShareIndex); err != nil {
			return fmt.Errorf("validating Sample: %w", err)
		}
		sb.Container = &cntr
		// NOTE: We don't have to validate Block in the RowBlock, as it's implicitly verified by string
		// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
		// verification)
		return nil
	}
}
