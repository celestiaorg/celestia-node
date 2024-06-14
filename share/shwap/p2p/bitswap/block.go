package bitswap

import (
	"context"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"

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
	// Marshal serializes bytes of Shwap Container the Block holds.
	// Must not include the Shwap ID.
	Marshal() ([]byte, error)
	// Populate fills up the Block with the Shwap container getting it out of the EDS
	// Accessor.
	Populate(context.Context, eds.Accessor) error
	// IsEmpty reports whether the Block been populated with Shwap container.
	// If the Block is empty, it can be populated with Fetch.
	IsEmpty() bool
	// PopulateFn returns closure that fills up the Block with Shwap container.
	// Population involves data validation against the Root.
	PopulateFn(*share.Root) PopulateFn
}
