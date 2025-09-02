package bitswap

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

const (
	// rowCodec is a CID codec used for row Bitswap requests over Namespaced Merkle Tree.
	rowCodec = 0x7800

	// rowMultihashCode is the multihash code for custom axis sampling multihash function.
	rowMultihashCode = 0x7801
)

// maxRowSize is the maximum size of the RowBlock.
// It is calculated as half of the square size multiplied by the share size.
var maxRowSize = share.MaxSquareSize / 2 * libshare.ShareSize

func init() {
	registerBlock(
		rowMultihashCode,
		rowCodec,
		maxRowSize,
		shwap.RowIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptyRowBlockFromCID(cid)
		},
	)
}

// RowBlock is a Bitswap compatible block for Shwap's Row container.
type RowBlock struct {
	ID shwap.RowID

	Container shwap.Row
}

// NewEmptyRowBlock constructs a new empty RowBlock.
func NewEmptyRowBlock(height uint64, rowIdx, edsSize int) (*RowBlock, error) {
	id, err := shwap.NewRowID(height, rowIdx, edsSize)
	if err != nil {
		return nil, err
	}

	return &RowBlock{ID: id}, nil
}

// EmptyRowBlockFromCID constructs an empty RowBlock out of the CID.
func EmptyRowBlockFromCID(cid cid.Cid) (*RowBlock, error) {
	ridData, err := extractFromCID(cid)
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
	return encodeToCID(rb.ID, rowMultihashCode, rowCodec)
}

func (rb *RowBlock) Height() uint64 {
	return rb.ID.Height()
}

func (rb *RowBlock) Marshal() ([]byte, error) {
	if rb.Container.IsEmpty() {
		return nil, fmt.Errorf("cannot marshal empty RowBlock")
	}

	container := rb.Container.ToProto()
	containerData, err := container.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowBlock container: %w", err)
	}

	return containerData, nil
}

func (rb *RowBlock) Populate(ctx context.Context, eds eds.Accessor) error {
	half, err := eds.AxisHalf(ctx, rsmt2d.Row, rb.ID.RowIndex)
	if err != nil {
		return fmt.Errorf("accessing Row AxisHalf: %w", err)
	}

	rb.Container = half.ToRow()
	return nil
}

func (rb *RowBlock) UnmarshalFn(root *share.AxisRoots) UnmarshalFn {
	return func(cntrData, idData []byte) error {
		if !rb.Container.IsEmpty() {
			return nil
		}

		rid, err := shwap.RowIDFromBinary(idData)
		if err != nil {
			return fmt.Errorf("unmarhaling RowID: %w", err)
		}

		if !rb.ID.Equals(rid) {
			return fmt.Errorf("requested %+v doesnt match given %+v", rb.ID, rid)
		}

		var row shwappb.Row
		if err := row.Unmarshal(cntrData); err != nil {
			return fmt.Errorf("unmarshaling Row for %+v: %w", rb.ID, err)
		}

		cntr, err := shwap.RowFromProto(&row)
		if err != nil {
			return fmt.Errorf("unmarshaling Row: %w", err)
		}

		if err = cntr.Verify(root, rb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating Row for %+v: %w", rb.ID, err)
		}

		rb.Container = cntr
		return nil
	}
}
