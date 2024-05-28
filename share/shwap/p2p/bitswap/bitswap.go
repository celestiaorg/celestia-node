package bitswap

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logger.Logger("shwap/bitswap")

// TODO:
//  * Synchronization for Fetch
//  * Test with race and count 100
//  * Hasher test
//  * Coverage
//  * godoc
//    * document steps required to add new id/container type

// PopulateFn is a closure that validates given bytes and populates
// Blocks with serialized shwap container in those bytes on success.
type PopulateFn func([]byte) error

type Block interface {
	blockBuilder

	// String returns string representation of the Block
	// to be used as map key. Might not be human-readable
	String() string
	// CID returns shwap ID of the Block formatted as CID.
	CID() cid.Cid
	// IsEmpty reports whether the Block has the shwap container.
	// If the Block is empty, it can be populated with Fetch.
	IsEmpty() bool
	// Populate returns closure that fills up the Block with shwap container.
	// Population involves data validation against the Root.
	Populate(*share.Root) PopulateFn
}

// Fetch
// Does not guarantee synchronization. Calling this func simultaneously with the same Block may
// cause issues. TODO: Describe motivation
func Fetch(ctx context.Context, fetcher exchange.Fetcher, root *share.Root, ids ...Block) error {
	cids := make([]cid.Cid, 0, len(ids))
	for _, id := range ids {
		if !id.IsEmpty() {
			continue
		}
		cids = append(cids, id.CID())

		idStr := id.String()
		populators.Store(idStr, id.Populate(root))
		defer populators.Delete(idStr)
	}

	// must start getting only after verifiers are registered
	blkCh, err := fetcher.GetBlocks(ctx, cids)
	if err != nil {
		return fmt.Errorf("fetching bitswap blocks: %w", err)
	}

	// GetBlocks handles ctx and closes blkCh, so we don't have to
	blks := make([]blocks.Block, 0, len(cids))
	for blk := range blkCh {
		blks = append(blks, blk)
	}
	if len(blks) != len(cids) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("not all the containers were found")
	}

	return nil
}

var populators sync.Map
