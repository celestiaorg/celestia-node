package bitswap

import (
	"context"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
)

var log = logger.Logger("shwap/bitswap")

// Block represents Bitswap compatible generalization over Shwap containers.
// All Shwap containers must have a registered wrapper
// implementing the interface in order to be compatible with Bitswap.
// NOTE: This is not a Blockchain block, but an IPFS/Bitswap block.
type Block interface {
	// CID returns Shwap ID of the Block formatted as CID.
	CID() cid.Cid
	// Height reports the Height of the Shwap container behind the Block.
	Height() uint64
	// Size reports expected size of the Block(without serialization overhead).
	// Must support getting size when the Block is not populated or empty and strive to
	// be low overhead.
	Size(context.Context, eds.Accessor) (int, error)

	// Populate fills up the Block with the Shwap container getting it out of the EDS
	// Accessor.
	Populate(context.Context, eds.Accessor) error
	// Marshal serializes bytes of the Shwap Container the Block holds.
	// MUST exclude the Shwap ID.
	Marshal() ([]byte, error)
	// UnmarshalFn returns closure that unmarshal the Block with the Shwap container.
	// Unmarshalling involves data validation against the given AxisRoots.
	UnmarshalFn(*share.AxisRoots) UnmarshalFn
}

// UnmarshalFn is a closure produced by a Block that unmarshalls and validates
// the given serialized bytes of a Shwap container with ID and populates the Block with it on success.
type UnmarshalFn func(container, id []byte) error
