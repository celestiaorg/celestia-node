package bitswap

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"

	bitswappb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logger.Logger("shwap/bitswap")

// PopulateFn is a closure produced by a Block that validates given
// serialized Shwap container and populates the Block with it on success.
type PopulateFn func([]byte) error

// Block represents Bitswap compatible Shwap container.
// All Shwap containers must have a registerBlock-ed wrapper
// implementing the interface to be compatible with Bitswap.
// NOTE: This is not Blockchain block, but IPFS/Bitswap block/
type Block interface {
	// CID returns Shwap ID of the Block formatted as CID.
	CID() cid.Cid
	// Height reports the Height the Shwap Container data behind the Block is from.
	Height() uint64
	// BlockFromEDS extract Bitswap Block out of the EDS.
	// TODO: Split into MarshalBinary and Populate
	BlockFromEDS(context.Context, eds.Accessor) (blocks.Block, error)

	// IsEmpty reports whether the Block been populated with Shwap container.
	// If the Block is empty, it can be populated with Fetch.
	IsEmpty() bool
	// PopulateFn returns closure that fills up the Block with Shwap container.
	// Population involves data validation against the Root.
	PopulateFn(*share.Root) PopulateFn
}

// toBlock converts given protobuf container into Bitswap Block.
func toBlock(cid cid.Cid, container proto.Marshaler) (blocks.Block, error) {
	containerData, err := container.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling container: %w", err)
	}

	blkProto := bitswappb.Block{
		Cid:       cid.Bytes(),
		Container: containerData,
	}

	blkData, err := blkProto.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling Block protobuf: %w", err)
	}

	blk, err := blocks.NewBlockWithCid(blkData, cid)
	if err != nil {
		return nil, fmt.Errorf("assembling Bitswap block: %w", err)
	}

	return blk, nil
}
