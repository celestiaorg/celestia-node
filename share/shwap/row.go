package shwap

import (
	"bytes"
	"fmt"

	blocks "github.com/ipfs/go-block-format"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// Row represents a Row of an EDS.
type Row struct {
	RowID

	// RowShares is the original non erasure-coded half of the Row.
	RowShares []share.Share
}

// NewRow constructs a new Row.
func NewRow(id RowID, axisHalf []share.Share) *Row {
	return &Row{
		RowID:     id,
		RowShares: axisHalf,
	}
}

// NewRowFromEDS constructs a new Row from the given EDS.
func NewRowFromEDS(
	height uint64,
	rowIdx int,
	square *rsmt2d.ExtendedDataSquare,
) (*Row, error) {
	sqrLn := int(square.Width())
	axisHalf := square.Row(uint(rowIdx))[:sqrLn/2]

	root, err := share.NewRoot(square)
	if err != nil {
		return nil, err
	}

	id, err := NewRowID(height, uint16(rowIdx), root)
	if err != nil {
		return nil, err
	}

	return NewRow(id, axisHalf), nil
}

// RowFromBlock converts blocks.Block into Row.
func RowFromBlock(blk blocks.Block) (*Row, error) {
	if err := validateCID(blk.Cid()); err != nil {
		return nil, err
	}

	s := &Row{}
	err := s.UnmarshalBinary(blk.RawData())
	if err != nil {
		return nil, fmt.Errorf("while unmarshalling Row: %w", err)
	}

	return s, nil
}

// IPLDBlock converts Row to an IPLD block for Bitswap compatibility.
func (r *Row) IPLDBlock() (blocks.Block, error) {
	data, err := r.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(data, r.Cid())
}

// MarshalBinary marshals Row to binary.
func (r *Row) MarshalBinary() ([]byte, error) {
	id, err := r.RowID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return (&shwappb.Row{
		RowId:   id,
		RowHalf: r.RowShares,
	}).Marshal()
}

// UnmarshalBinary unmarshal Row from binary.
func (r *Row) UnmarshalBinary(data []byte) error {
	proto := &shwappb.Row{}
	if err := proto.Unmarshal(data); err != nil {
		return err
	}

	err := r.RowID.UnmarshalBinary(proto.RowId)
	if err != nil {
		return err
	}

	r.RowShares = proto.RowHalf
	return nil
}

// Verify validates Row's fields and verifies Row inclusion.
func (r *Row) Verify(root *share.Root) error {
	if err := r.RowID.Verify(root); err != nil {
		return err
	}

	encoded, err := share.DefaultRSMT2DCodec().Encode(r.RowShares)
	if err != nil {
		return fmt.Errorf("while decoding erasure coded half: %w", err)
	}
	// TODO: encoded already contains all the shares initially [-len(RowShares):]
	r.RowShares = append(r.RowShares, encoded...)

	sqrLn := uint64(len(r.RowShares) / 2)
	tree := wrapper.NewErasuredNamespacedMerkleTree(sqrLn, uint(r.RowID.RowIndex))
	for _, shr := range r.RowShares {
		err := tree.Push(shr)
		if err != nil {
			return fmt.Errorf("while pushing shares to NMT: %w", err)
		}
	}

	rowRoot, err := tree.Root()
	if err != nil {
		return fmt.Errorf("while computing NMT root: %w", err)
	}

	if !bytes.Equal(root.RowRoots[r.RowIndex], rowRoot) {
		return fmt.Errorf("invalid RowHash: %X != %X", root, root.RowRoots[r.RowIndex])
	}

	return nil
}
