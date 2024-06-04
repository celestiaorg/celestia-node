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
	// rowCodec is a CID codec used for row Bitswap requests over Namespaced Merkle Tree.
	rowCodec = 0x7800

	// rowMultihashCode is the multihash code for custom axis sampling multihash function.
	rowMultihashCode = 0x7801
)

func init() {
	registerBlock(
		rowMultihashCode,
		rowCodec,
		shwap.RowIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptyRowBlockFromCID(cid)
		},
	)
}

// RowBlock is a Bitswap compatible block for Shwap's Row container.
type RowBlock struct {
	ID        shwap.RowID
	Container *shwap.Row
}

// NewEmptyRowBlock constructs a new empty RowBlock.
func NewEmptyRowBlock(height uint64, rowIdx int, root *share.Root) (*RowBlock, error) {
	id, err := shwap.NewRowID(height, rowIdx, root)
	if err != nil {
		return nil, err
	}

	return &RowBlock{ID: id}, nil
}

// EmptyRowBlockFromCID constructs an empty RowBlock out of the CID.
func EmptyRowBlockFromCID(cid cid.Cid) (*RowBlock, error) {
	ridData, err := extractCID(cid)
	if err != nil {
		return nil, err
	}

	rid, err := shwap.RowIDFromBinary(ridData)
	if err != nil {
		return nil, fmt.Errorf("while unmarhaling RowBlock: %w", err)
	}
	return &RowBlock{ID: rid}, nil
}

func (rb *RowBlock) CID() cid.Cid {
	return encodeCID(rb.ID, rowMultihashCode, rowCodec)
}

func (rb *RowBlock) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	row := shwap.RowFromEDS(eds, rb.ID.RowIndex, shwap.Left)

	cid := rb.CID()
	rowBlk := bitswappb.RowBlock{
		RowCid: cid.Bytes(),
		Row:    row.ToProto(),
	}

	blkData, err := rowBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(blkData, cid)
	if err != nil {
		return nil, fmt.Errorf("assembling Bitswap block: %w", err)
	}

	return blk, nil
}

func (rb *RowBlock) IsEmpty() bool {
	return rb.Container == nil
}

func (rb *RowBlock) PopulateFn(root *share.Root) PopulateFn {
	return func(data []byte) error {
		if !rb.IsEmpty() {
			return nil
		}
		var rowBlk bitswappb.RowBlock
		if err := rowBlk.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling RowBlock: %w", err)
		}

		cntr := shwap.RowFromProto(rowBlk.Row)
		if err := cntr.Validate(root, rb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating Row: %w", err)
		}
		rb.Container = &cntr

		// NOTE: We don't have to validate ID in the RowBlock, as it's implicitly verified by string
		// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
		// verification)
		return nil
	}
}
