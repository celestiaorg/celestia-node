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
	RegisterID(
		rowNamespaceDataMultihashCode,
		rowNamespaceDataCodec,
		shwap.RowNamespaceDataIDSize,
		func(cid cid.Cid) (blockBuilder, error) {
			return RowNamespaceDataIDFromCID(cid)
		},
	)
}

type RowNamespaceDataID shwap.RowNamespaceDataID

// RowNamespaceDataIDFromCID coverts CID to RowNamespaceDataID.
func RowNamespaceDataIDFromCID(cid cid.Cid) (RowNamespaceDataID, error) {
	rndidData, err := extractCID(cid)
	if err != nil {
		return RowNamespaceDataID{}, err
	}

	rndid, err := shwap.RowNamespaceDataIDFromBinary(rndidData)
	if err != nil {
		return RowNamespaceDataID{}, fmt.Errorf("unmarhalling RowNamespaceDataID: %w", err)
	}

	return RowNamespaceDataID(rndid), nil
}

func (rndid RowNamespaceDataID) String() string {
	data, err := rndid.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("marshaling RowNamespaceDataID: %w", err))
	}

	return string(data)
}

func (rndid RowNamespaceDataID) MarshalBinary() ([]byte, error) {
	return shwap.RowNamespaceDataID(rndid).MarshalBinary()
}

func (rndid RowNamespaceDataID) CID() cid.Cid {
	return encodeCID(shwap.RowNamespaceDataID(rndid), rowNamespaceDataMultihashCode, rowNamespaceDataCodec)
}

func (rndid RowNamespaceDataID) BlockFromEDS(eds *rsmt2d.ExtendedDataSquare) (blocks.Block, error) {
	rnd, err := shwap.RowNamespaceDataFromEDS(eds, rndid.DataNamespace, rndid.RowIndex)
	if err != nil {
		return nil, err
	}

	dataID, err := rndid.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowNamespaceDataID: %w", err)
	}

	rndidBlk := bitswapb.RowNamespaceDataBlock{
		RowNamespaceDataId: dataID,
		Data:               rnd.ToProto(),
	}

	dataBlk, err := rndidBlk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling RowNamespaceDataBlock: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(dataBlk, rndid.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling block: %w", err)
	}

	return blk, nil
}

func (rndid RowNamespaceDataID) UnmarshalContainer(root *share.Root, data []byte) (shwap.RowNamespaceData, error) {
	var rndBlk bitswapb.RowNamespaceDataBlock
	if err := rndBlk.Unmarshal(data); err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("unmarshaling RowNamespaceDataBlock: %w", err)
	}

	rnd := shwap.RowNamespaceDataFromProto(rndBlk.Data)
	if err := rnd.Validate(root, rndid.DataNamespace, rndid.RowIndex); err != nil {
		return shwap.RowNamespaceData{}, fmt.Errorf("validating RowNamespaceData: %w", err)
	}
	// NOTE: We don't have to validate ID in the RowBlock, as it's implicitly verified by string
	// equality of globalVerifiers entry key(requesting side) and hasher accessing the entry(response
	// verification)
	return rnd, nil
}
