package shwap

import (
	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// Row represents a Row of an EDS.
type Row struct {
	RowID

	// RowShares is the original non erasure-coded half of the Row.
	RowShares share.Row
}

// NewRow constructs a new Row.
func NewRow(id RowID, halfAxis share.Row) *Row {
	return &Row{
		RowID:     id,
		RowShares: halfAxis,
	}
}

// RowFromBlock converts blocks.Block into Row.
func RowFromBlock(blk blocks.Block) (*Row, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	var row shwappb.RowBlock
	if err := row.Unmarshal(blk.RawData()); err != nil {
		return nil, err
	}
	return RowFromProto(&row)
}

// IPLDBlock converts Row to an IPLD block for Bitswap compatibility.
func (r *Row) IPLDBlock() (blocks.Block, error) {
	data, err := r.ToProto().Marshal()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, r.Cid())
}

// MarshalBinary marshals Row to binary.
func (r *Row) ToProto() *shwappb.RowBlock {
	return &shwappb.RowBlock{
		RowId: r.RowID.MarshalBinary(),
		Row:   r.RowShares.ToProto(),
	}
}

// Verify validates Row's fields and verifies Row inclusion.
func (r *Row) Verify(root *share.Root) error {
	if err := r.RowID.Verify(root); err != nil {
		return err
	}
	shares, err := r.RowShares.Shares()
	if err != nil {
		return err
	}
	return shares.Validate(root, int(r.RowIndex))
}

func RowFromProto(rowProto *shwappb.RowBlock) (*Row, error) {
	id, err := RowIDFromBinary(rowProto.RowId)
	if err != nil {
		return nil, err
	}
	return &Row{
		RowID:     id,
		RowShares: share.RowFromProto(rowProto.Row),
	}, nil
}
