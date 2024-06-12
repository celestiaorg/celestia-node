package bitswap

import (
	"fmt"
	"sync/atomic"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

const (
	// sampleCodec is a CID codec used for share sampling Bitswap requests over Namespaced
	// Merkle Tree.
	sampleCodec = 0x7810

	// sampleMultihashCode is the multihash code for share sampling multihash function.
	sampleMultihashCode = 0x7811
)

func init() {
	registerBlock(
		sampleMultihashCode,
		sampleCodec,
		shwap.SampleIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptySampleBlockFromCID(cid)
		},
	)
}

// SampleBlock is a Bitswap compatible block for Shwap's Sample container.
type SampleBlock struct {
	ID        shwap.SampleID
	container atomic.Pointer[shwap.Sample]
}

// NewEmptySampleBlock constructs a new empty SampleBlock.
func NewEmptySampleBlock(height uint64, rowIdx, colIdx int, root *share.Root) (*SampleBlock, error) {
	id, err := shwap.NewSampleID(height, rowIdx, colIdx, root)
	if err != nil {
		return nil, err
	}

	return &SampleBlock{ID: id}, nil
}

// EmptySampleBlockFromCID constructs an empty SampleBlock out of the CID.
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

func (sb *SampleBlock) CID() cid.Cid {
	return encodeCID(sb.ID, sampleMultihashCode, sampleCodec)
}

func (sb *SampleBlock) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	smpl, err := shwap.SampleFromEDS(eds, rsmt2d.Row, sb.ID.RowIndex, sb.ID.ShareIndex)
	if err != nil {
		return nil, err
	}

	blk, err := toBlock(sb.CID(), smpl.ToProto())
	if err != nil {
		return nil, fmt.Errorf("converting Sample to Bitswap block: %w", err)
	}

	return blk, nil
}

func (sb *SampleBlock) IsEmpty() bool {
	return sb.Container() == nil
}

func (sb *SampleBlock) Container() *shwap.Sample {
	return sb.container.Load()
}

func (sb *SampleBlock) PopulateFn(root *share.Root) PopulateFn {
	return func(data []byte) error {
		if !sb.IsEmpty() {
			return nil
		}
		var sample shwappb.Sample
		if err := sample.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling Sample: %w", err)
		}

		cntr := shwap.SampleFromProto(&sample)
		if err := cntr.Validate(root, sb.ID.RowIndex, sb.ID.ShareIndex); err != nil {
			return fmt.Errorf("validating Sample: %w", err)
		}
		sb.container.Store(&cntr)
		return nil
	}
}
