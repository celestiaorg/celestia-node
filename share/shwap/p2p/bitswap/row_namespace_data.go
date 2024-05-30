package bitswap

import (
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
	bitswapb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"
)

const (
	// rowNamespaceDataCodec is a CID codec used for data Bitswap requests over Namespaced Merkle Tree.
	rowNamespaceDataCodec = 0x7820

	// rowNamespaceDataMultihashCode is the multihash code for data multihash function.
	rowNamespaceDataMultihashCode = 0x7821
)

func init() {
	RegisterBlock(
		rowNamespaceDataMultihashCode,
		rowNamespaceDataCodec,
		shwap.RowNamespaceDataIDSize,
		func(cid cid.Cid) (blockBuilder, error) {
			return EmptyRowNamespaceDataBlockFromCID(cid)
		},
	)
}

type RowNamespaceDataBlock struct {
	ID        shwap.RowNamespaceDataID
	Container *shwap.RowNamespaceData
}

func NewEmptyRowNamespaceDataBlock(
	height uint64,
	rowIdx int,
	namespace share.Namespace,
	root *share.Root,
) (*RowNamespaceDataBlock, error) {
	id, err := shwap.NewRowNamespaceDataID(height, rowIdx, namespace, root)
	if err != nil {
		return nil, err
	}

	return &RowNamespaceDataBlock{ID: id}, nil
}

// EmptyRowNamespaceDataBlockFromCID coverts CID to RowNamespaceDataBlock.
func EmptyRowNamespaceDataBlockFromCID(cid cid.Cid) (*RowNamespaceDataBlock, error) {
	rndidData, err := extractCID(cid)
	if err != nil {
		return nil, err
	}

	rndid, err := shwap.RowNamespaceDataIDFromBinary(rndidData)
	if err != nil {
		return nil, fmt.Errorf("unmarhalling RowNamespaceDataBlock: %w", err)
	}

	return &RowNamespaceDataBlock{ID: rndid}, nil
}

func (rndb *RowNamespaceDataBlock) IsEmpty() bool {
	return rndb.Container == nil
}

func (rndb *RowNamespaceDataBlock) String() string {
	data, err := rndb.ID.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling RowNamespaceDataBlock: %w", err))
	}

	return string(data)
}

func (rndb *RowNamespaceDataBlock) CID() cid.Cid {
	return encodeCID(rndb.ID, rowNamespaceDataMultihashCode, rowNamespaceDataCodec)
}

func (rndb *RowNamespaceDataBlock) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	rnd, err := shwap.RowNamespaceDataFromEDS(eds, rndb.ID.DataNamespace, rndb.ID.RowIndex)
	if err != nil {
		return nil, err
	}

	rndID, err := rndb.ID.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowNamespaceDataBlock: %w", err)
	}

	rndidBlk := bitswapb.RowNamespaceDataBlock{
		RowNamespaceDataId: rndID,
		Data:               rnd.ToProto(),
	}

	blkData, err := rndidBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowNamespaceDataBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(blkData, rndb.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling block: %w", err)
	}

	return blk, nil
}

func (rndb *RowNamespaceDataBlock) Populate(root *share.Root) PopulateFn {
	return func(data []byte) error {
		var rndBlk bitswapb.RowNamespaceDataBlock
		if err := rndBlk.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling RowNamespaceDataBlock: %w", err)
		}

		cntr := shwap.RowNamespaceDataFromProto(rndBlk.Data)
		if err := cntr.Validate(root, rndb.ID.DataNamespace, rndb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating RowNamespaceData: %w", err)
		}
		rndb.Container = &cntr

		// NOTE: We don't have to validate Block in the RowBlock, as it's implicitly verified by string
		// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
		// verification)
		return nil
	}
}