package bitswap

import (
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logger.Logger("shwap/bitswap")

// PopulateFn is a closure produced by a Block that validates given
// serialized Shwap container and populates the Block with it on success.
type PopulateFn func([]byte) error

// Block represents Bitswap compatible Shwap container.
// All Shwap containers must have a RegisterBlock-ed wrapper
// implementing the interface to be compatible with Bitswap.
type Block interface {
	// String returns string representation of the Block
	// to be used as map key. Might not be human-readable.
	String() string

	// CID returns Shwap ID of the Block formatted as CID.
	CID() cid.Cid
	// BlockFromEDS extract Bitswap Block out of the EDS.
	BlockFromEDS(*rsmt2d.ExtendedDataSquare) (blocks.Block, error)

	// IsEmpty reports whether the Block been populated with Shwap container.
	// If the Block is empty, it can be populated with Fetch.
	IsEmpty() bool
	// Populate returns closure that fills up the Block with Shwap container.
	// Population involves data validation against the Root.
	Populate(*share.Root) PopulateFn
}
