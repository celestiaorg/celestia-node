package bitswap

import (
	"context"
	"fmt"
	"sync/atomic"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
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
	ID shwap.RowID

	container atomic.Pointer[shwap.Row]
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

func (rb *RowBlock) BlockFromEDS(ctx context.Context, eds eds.Accessor) (blocks.Block, error) {
	half, err := eds.AxisHalf(ctx, rsmt2d.Row, rb.ID.RowIndex)
	if err != nil {
		return nil, fmt.Errorf("getting Row AxisHalf: %w", err)
	}

	blk, err := toBlock(rb.CID(), half.ToRow().ToProto())
	if err != nil {
		return nil, fmt.Errorf("converting Row to Bitswap block: %w", err)
	}

	return blk, nil
}

func (rb *RowBlock) IsEmpty() bool {
	return rb.Container() == nil
}

func (rb *RowBlock) Container() *shwap.Row {
	return rb.container.Load()
}

func (rb *RowBlock) PopulateFn(root *share.Root) PopulateFn {
	return func(data []byte) error {
		if !rb.IsEmpty() {
			return nil
		}
		var row shwappb.Row
		if err := row.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling Row: %w", err)
		}

		cntr := shwap.RowFromProto(&row)
		if err := cntr.Validate(root, rb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating Row: %w", err)
		}
		rb.container.Store(&cntr)
		return nil
	}
}
