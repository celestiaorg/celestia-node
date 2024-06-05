package bitswap

import (
	"fmt"
	"sync/atomic"

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
	registerBlock(
		rowNamespaceDataMultihashCode,
		rowNamespaceDataCodec,
		shwap.RowNamespaceDataIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptyRowNamespaceDataBlockFromCID(cid)
		},
	)
}

// RowNamespaceDataBlock is a Bitswap compatible block for Shwap's RowNamespaceData container.
type RowNamespaceDataBlock struct {
	ID shwap.RowNamespaceDataID

	container atomic.Pointer[shwap.RowNamespaceData]
}

// NewEmptyRowNamespaceDataBlock constructs a new empty RowNamespaceDataBlock.
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

// EmptyRowNamespaceDataBlockFromCID constructs an empty RowNamespaceDataBlock out of the CID.
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

func (rndb *RowNamespaceDataBlock) CID() cid.Cid {
	return encodeCID(rndb.ID, rowNamespaceDataMultihashCode, rowNamespaceDataCodec)
}

func (rndb *RowNamespaceDataBlock) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	rnd, err := shwap.RowNamespaceDataFromEDS(eds, rndb.ID.DataNamespace, rndb.ID.RowIndex)
	if err != nil {
		return nil, err
	}

	cid := rndb.CID()
	rndBlk := bitswapb.RowNamespaceDataBlock{
		RowNamespaceDataCid: cid.Bytes(),
		Data:                rnd.ToProto(),
	}

	blkData, err := rndBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowNamespaceDataBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(blkData, cid)
	if err != nil {
		return nil, fmt.Errorf("assembling Bitswap block: %w", err)
	}

	return blk, nil
}

func (rndb *RowNamespaceDataBlock) IsEmpty() bool {
	return rndb.Container() == nil
}

func (rndb *RowNamespaceDataBlock) Container() *shwap.RowNamespaceData {
	return rndb.container.Load()
}

func (rndb *RowNamespaceDataBlock) PopulateFn(root *share.Root) PopulateFn {
	return func(data []byte) error {
		if !rndb.IsEmpty() {
			return nil
		}
		var rndBlk bitswapb.RowNamespaceDataBlock
		if err := rndBlk.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling RowNamespaceDataBlock: %w", err)
		}

		cntr := shwap.RowNamespaceDataFromProto(rndBlk.Data)
		if err := cntr.Validate(root, rndb.ID.DataNamespace, rndb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating RowNamespaceData: %w", err)
		}
		rndb.container.Store(&cntr)

		// NOTE: We don't have to validate ID in the RowBlock, as it's implicitly verified by string
		// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
		// verification)
		return nil
	}
}
