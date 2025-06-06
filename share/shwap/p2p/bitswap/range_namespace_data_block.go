package bitswap

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	shwappb "github.com/celestiaorg/celestia-node/share/shwap/pb"
)

const (
	// rangeNamespaceDataCodec is a CID codec used for data Bitswap requests over Namespaced Merkle Tree.
	rangeNamespaceDataCodec = 0x7830

	// rangeNamespaceDataMultihashCode is the multihash code for data multihash function.
	rangeNamespaceDataMultihashCode = 0x7831
)

// maxRangeSize is the maximum size of the RangeNamespaceDataBlock.
// It is calculated as a maxRowSize multiplied by the half of the square size.
var maxRangeSize = maxRowSize * share.MaxSquareSize / 2

func init() {
	registerBlock(
		rangeNamespaceDataMultihashCode,
		rangeNamespaceDataCodec,
		maxRangeSize,
		shwap.RangeNamespaceDataIDSize,
		func(cid cid.Cid) (Block, error) {
			return EmptyRangeNamespaceDataBlockFromCID(cid)
		},
	)
}

// RangeNamespaceDataBlock is a Bitswap compatible block for Shwap's RangeNamespaceData container.
type RangeNamespaceDataBlock struct {
	ID shwap.RangeNamespaceDataID

	Container shwap.RangeNamespaceData
}

// NewEmptyRangeNamespaceDataBlock constructs a new empty RangeNamespaceDataBlock.
func NewEmptyRangeNamespaceDataBlock(
	height uint64,
	from, to, odsSize int,
) (*RangeNamespaceDataBlock, error) {
	id, err := shwap.NewRangeNamespaceDataID(shwap.EdsID{Height: height}, from, to, odsSize)
	if err != nil {
		return nil, err
	}

	return &RangeNamespaceDataBlock{ID: id}, nil
}

// EmptyRangeNamespaceDataBlockFromCID constructs an empty RangeNamespaceDataBlock out of the CID.
func EmptyRangeNamespaceDataBlockFromCID(cid cid.Cid) (*RangeNamespaceDataBlock, error) {
	rngidData, err := extractFromCID(cid)
	if err != nil {
		return nil, err
	}

	rndid, err := shwap.RangeNamespaceDataIDFromBinary(rngidData)
	if err != nil {
		return nil, fmt.Errorf("unmarhalling RangeNamespaceDataBlock: %w", err)
	}
	return &RangeNamespaceDataBlock{ID: rndid}, nil
}

func (rndb *RangeNamespaceDataBlock) CID() cid.Cid {
	return encodeToCID(rndb.ID, rangeNamespaceDataMultihashCode, rangeNamespaceDataCodec)
}

func (rndb *RangeNamespaceDataBlock) Height() uint64 {
	return rndb.ID.Height()
}

func (rndb *RangeNamespaceDataBlock) Marshal() ([]byte, error) {
	if rndb.Container.IsEmpty() {
		return nil, fmt.Errorf("cannot marshal empty RangeNamespaceDataBlock")
	}
	container := rndb.Container.ToProto()
	containerData, err := container.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RangeNamespaceDataBlock container: %w", err)
	}
	return containerData, nil
}

func (rndb *RangeNamespaceDataBlock) Populate(ctx context.Context, eds eds.Accessor) error {
	rnd, err := eds.RangeNamespaceData(
		ctx,
		rndb.ID.From,
		rndb.ID.To,
	)
	if err != nil {
		return fmt.Errorf("accessing RangeNamespaceData: %w", err)
	}
	rndb.Container = rnd
	return nil
}

func (rndb *RangeNamespaceDataBlock) UnmarshalFn(root *share.AxisRoots) UnmarshalFn {
	return func(cntrData, idData []byte) error {
		if !rndb.Container.IsEmpty() {
			return nil
		}
		rndid, err := shwap.RangeNamespaceDataIDFromBinary(idData)
		if err != nil {
			return fmt.Errorf("unmarhaling RangeNamespaceDataID: %w", err)
		}

		odsSize := len(root.RowRoots) / 2

		if err = rndid.Verify(odsSize); err != nil {
			return fmt.Errorf("verifying RangeNamespaceDataID: %w", err)
		}

		if !rndb.ID.Equals(rndid) {
			return fmt.Errorf("requested %+v doesnt match given %+v", rndb.ID, rndid)
		}

		var rnd shwappb.RangeNamespaceData
		if err := rnd.Unmarshal(cntrData); err != nil {
			return fmt.Errorf("unmarshaling RangeNamespaceData for %+v: %w", rndb.ID, err)
		}

		rangeNsData, err := shwap.RangeNamespaceDataFromProto(&rnd)
		if err != nil {
			return fmt.Errorf("unmarshaling RangeNamespaceData for %+v: %w", rndb.ID, err)
		}

		from, err := shwap.SampleCoordsFrom1DIndex(rndid.From, odsSize)
		if err != nil {
			return fmt.Errorf("converting from index to coordinates for %+v: %w", rndb.ID, err)
		}
		to, err := shwap.SampleCoordsFrom1DIndex(rndid.To-1, odsSize)
		if err != nil {
			return fmt.Errorf("converting to index to coordinates for %+v: %w", rndb.ID, err)
		}

		if err = rangeNsData.VerifyInclusion(
			from, to,
			odsSize,
			root.RowRoots[from.Row:to.Row+1],
		); err != nil {
			return fmt.Errorf("validating RangeNamespaceData for %+v: %w", rndb.ID, err)
		}

		rndb.Container = rangeNsData
		return nil
	}
}
