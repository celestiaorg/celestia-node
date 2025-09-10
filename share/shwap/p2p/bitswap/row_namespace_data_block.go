package bitswap

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

const (
	// rowNamespaceDataCodec is a CID codec used for data Bitswap requests over Namespaced Merkle Tree.
	rowNamespaceDataCodec = 0x7820

	// rowNamespaceDataMultihashCode is the multihash code for data multihash function.
	rowNamespaceDataMultihashCode = 0x7821
)

// maxRNDSize is the maximum size of the RowNamespaceDataBlock.
var maxRNDSize = maxRowSize

func init() {
	registerBlock(
		rowNamespaceDataMultihashCode,
		rowNamespaceDataCodec,
		maxRNDSize,
		shwap.RowNamespaceDataIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptyRowNamespaceDataBlockFromCID(cid)
		},
	)
}

// RowNamespaceDataBlock is a Bitswap compatible block for Shwap's RowNamespaceData container.
type RowNamespaceDataBlock struct {
	ID shwap.RowNamespaceDataID

	Container shwap.RowNamespaceData
}

// NewEmptyRowNamespaceDataBlock constructs a new empty RowNamespaceDataBlock.
func NewEmptyRowNamespaceDataBlock(
	height uint64,
	rowIdx int,
	namespace libshare.Namespace,
	edsSize int,
) (*RowNamespaceDataBlock, error) {
	id, err := shwap.NewRowNamespaceDataID(height, rowIdx, namespace, edsSize)
	if err != nil {
		return nil, err
	}

	return &RowNamespaceDataBlock{ID: id}, nil
}

// EmptyRowNamespaceDataBlockFromCID constructs an empty RowNamespaceDataBlock out of the CID.
func EmptyRowNamespaceDataBlockFromCID(cid cid.Cid) (*RowNamespaceDataBlock, error) {
	rndidData, err := extractFromCID(cid)
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
	return encodeToCID(rndb.ID, rowNamespaceDataMultihashCode, rowNamespaceDataCodec)
}

func (rndb *RowNamespaceDataBlock) Height() uint64 {
	return rndb.ID.Height()
}

func (rndb *RowNamespaceDataBlock) Marshal() ([]byte, error) {
	if rndb.Container.IsEmpty() {
		return nil, fmt.Errorf("cannot marshal empty RowNamespaceDataBlock")
	}

	container := rndb.Container.ToProto()
	containerData, err := container.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowNamespaceDataBlock container: %w", err)
	}

	return containerData, nil
}

func (rndb *RowNamespaceDataBlock) Populate(ctx context.Context, eds eds.Accessor) error {
	rnd, err := eds.RowNamespaceData(ctx, rndb.ID.DataNamespace, rndb.ID.RowIndex)
	if err != nil {
		return fmt.Errorf("accessing RowNamespaceData: %w", err)
	}

	rndb.Container = rnd
	return nil
}

func (rndb *RowNamespaceDataBlock) UnmarshalFn(root *share.AxisRoots) UnmarshalFn {
	return func(cntrData, idData []byte) error {
		if !rndb.Container.IsEmpty() {
			return nil
		}

		rndid, err := shwap.RowNamespaceDataIDFromBinary(idData)
		if err != nil {
			return fmt.Errorf("unmarhaling RowNamespaceDataID: %w", err)
		}

		if !rndb.ID.Equals(rndid) {
			return fmt.Errorf("requested %+v doesnt match given %+v", rndb.ID, rndid)
		}

		var rnd shwappb.RowNamespaceData
		if err := rnd.Unmarshal(cntrData); err != nil {
			return fmt.Errorf("unmarshaling RowNamespaceData for %+v: %w", rndb.ID, err)
		}

		cntr, err := shwap.RowNamespaceDataFromProto(&rnd)
		if err != nil {
			return fmt.Errorf("unmarshaling RowNamespaceData for %+v: %w", rndb.ID, err)
		}
		if err := cntr.Verify(root, rndb.ID.DataNamespace, rndb.ID.RowIndex); err != nil {
			return fmt.Errorf("validating RowNamespaceData for %+v: %w", rndb.ID, err)
		}

		rndb.Container = cntr
		return nil
	}
}
