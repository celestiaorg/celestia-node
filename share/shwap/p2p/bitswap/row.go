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
	RegisterBlock(
		rowMultihashCode,
		rowCodec,
		shwap.RowIDSize,
		func(cid cid.Cid) (blockBuilder, error) {
			return EmptyRowBlockFromCID(cid)
		},
	)
}

type RowBlock struct {
	ID        shwap.RowID
	Container *shwap.Row
}

func NewEmptyRowBlock(height uint64, rowIdx int, root *share.Root) (*RowBlock, error) {
	id, err := shwap.NewRowID(height, rowIdx, root)
	if err != nil {
		return nil, err
	}

	return &RowBlock{ID: id}, nil
}

// EmptyRowBlockFromCID coverts CID to RowBlock.
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

func (rb *RowBlock) IsEmpty() bool {
	return rb.Container == nil
}

func (rb *RowBlock) String() string {
	data, err := rb.ID.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling RowBlock: %w", err))
	}
	return string(data)
}

func (rb *RowBlock) CID() cid.Cid {
	return encodeCID(rb.ID, rowMultihashCode, rowCodec)
}

func (rb *RowBlock) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	row := shwap.RowFromEDS(eds, rb.ID.RowIndex, shwap.Left)

	rowID, err := rb.ID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowBlock: %w", err)
	}

	rowBlk := bitswappb.RowBlock{
		RowId: rowID,
		Row:   row.ToProto(),
	}

	blkData, err := rowBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(blkData, rb.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling block: %w", err)
	}

	return blk, nil
}

func (rb *RowBlock) Populate(root *share.Root) PopulateFn {
	return func(data []byte) error {
		var rowBlk bitswappb.RowBlock
		if err := rowBlk.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling RowBlock: %w", err)
		}

		cntr := shwap.RowFromProto(rowBlk.Row)
		if err := cntr.Validate(root, rb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating Row: %w", err)
		}
		rb.Container = &cntr

		// NOTE: We don't have to validate Block in the RowBlock, as it's implicitly verified by string
		// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
		// verification)
		return nil
	}
}
