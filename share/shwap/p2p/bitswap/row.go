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
	RegisterID(
		rowMultihashCode,
		rowCodec,
		shwap.RowIDSize,
		func(cid cid.Cid) (blockBuilder, error) {
			return RowIDFromCID(cid)
		},
	)
}

type RowID shwap.RowID

// RowIDFromCID coverts CID to RowID.
func RowIDFromCID(cid cid.Cid) (RowID, error) {
	ridData, err := extractCID(cid)
	if err != nil {
		return RowID{}, err
	}

	rid, err := shwap.RowIDFromBinary(ridData)
	if err != nil {
		return RowID{}, fmt.Errorf("while unmarhaling RowID: %w", err)
	}
	return RowID(rid), nil
}

func (rid RowID) String() string {
	data, err := rid.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling RowID: %w", err))
	}
	return string(data)
}

func (rid RowID) MarshalBinary() ([]byte, error) {
	return shwap.RowID(rid).MarshalBinary()
}

func (rid RowID) CID() cid.Cid {
	return encodeCID(rid, rowMultihashCode, rowCodec)
}

func (rid RowID) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	row := shwap.RowFromEDS(eds, rid.RowIndex, shwap.Left)

	dataID, err := rid.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowID: %w", err)
	}

	rowBlk := bitswappb.RowBlock{
		RowId: dataID,
		Row:   row.ToProto(),
	}

	dataBlk, err := rowBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(dataBlk, rid.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling block: %w", err)
	}

	return blk, nil
}

type RowBlock struct {
	RowID
	Row shwap.Row
}

func (r *RowBlock) Verifier(root *share.Root) verify {
	return func(data []byte) error {
		var rowBlk bitswappb.RowBlock
		if err := rowBlk.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling RowBlock: %w", err)
		}

		r.Row = shwap.RowFromProto(rowBlk.Row)
		if err := r.Row.Validate(root, r.RowID.RowIndex); err != nil {
			fmt.Errorf("validating Row: %w", err)
		}

		// NOTE: We don't have to validate ID in the RowBlock, as it's implicitly verified by string
		// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
		// verification)
		return nil
	}
}
