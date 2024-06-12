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

	blk, err := toBlock(rndb.CID(), rnd.ToProto())
	if err != nil {
		return nil, fmt.Errorf("converting RowNamespaceData to Bitswap block: %w", err)
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
		var rnd shwappb.RowNamespaceData
		if err := rnd.Unmarshal(data); err != nil {
			return fmt.Errorf("unmarshaling RowNamespaceData: %w", err)
		}

		cntr := shwap.RowNamespaceDataFromProto(&rnd)
		if err := cntr.Validate(root, rndb.ID.DataNamespace, rndb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating RowNamespaceData: %w", err)
		}
		rndb.container.Store(&cntr)
		return nil
	}
}
