package bitswap

import (
	"context"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

var log = logger.Logger("shwap/bitswap")

// Block represents Bitswap compatible Shwap container.
// All Shwap containers must have a registerBlock-ed wrapper
// implementing the interface to be compatible with Bitswap.
// NOTE: This is not Blockchain block, but IPFS/Bitswap block.
type Block interface {
	// CID returns Shwap ID of the Block formatted as CID.
	CID() cid.Cid
	// Height reports the Height the Shwap Container data behind the Block is from.
	Height() uint64

	// IsEmpty reports whether the Block holds respective Shwap container.
	IsEmpty() bool
	// Populate fills up the Block with the Shwap container getting it out of the EDS
	// Accessor.
	Populate(context.Context, eds.Accessor) error
	// Marshal serializes bytes of Shwap Container the Block holds.
	// Must not include the Shwap ID.
	Marshal() ([]byte, error)
	// UnmarshalFn returns closure that unmarshal the Block with Shwap container.
	// Unmarshalling involves data validation against the Root.
	UnmarshalFn(*share.Root) UnmarshalFn
}

// UnmarshalFn is a closure produced by a Block that unmarshalls and validates
// given serialized Shwap container and populates the Block with it on success.
type UnmarshalFn func([]byte) error
