package bitswap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share"
)

// Fetch fetches and populates given Blocks using Fetcher wrapping Bitswap.
//
// Validates Block against the given Root and skips Blocks that are already populated.
// Gracefully synchronize identical Blocks requested simultaneously.
// Blocks until either context is canceled or all Blocks are fetched and populated.
func Fetch(ctx context.Context, fetcher exchange.Fetcher, root *share.Root, blks ...Block) error {
	cids := make([]cid.Cid, 0, len(blks))
	duplicate := make(map[cid.Cid]Block)
	for _, blk := range blks {
		if !blk.IsEmpty() {
			continue // skip populated Blocks
		}

		idStr := blk.String()
		p := blk.PopulateFn(root)
		cid := blk.CID()
		cids = append(cids, cid)
		// store the PopulateFn s.t. hasher can access it
		// and fill in the Block
		_, exists := populatorFns.LoadOrStore(idStr, p)
		if exists {
			// in case there is ongoing fetch happening for the same Block elsewhere
			// and PopulateFn has already been set -- mark the Block as duplicate
			duplicate[cid] = blk
		} else {
			// only do the cleanup if we stored the PopulateFn
			defer populatorFns.Delete(idStr)
		}
	}

	blkCh, err := fetcher.GetBlocks(ctx, cids)
	if err != nil {
		return fmt.Errorf("fetching bitswap blocks: %w", err)
	}

	for blk := range blkCh { // GetBlocks closes blkCh on ctx cancellation
		// check if the blk is a duplicate
		id, ok := duplicate[blk.Cid()]
		if !ok {
			continue
		}
		// if it is, we have to populate it ourselves instead of hasher,
		// as there is only one PopulateFN allowed per ID
		err := id.PopulateFn(root)(blk.RawData())
		if err != nil {
			// this case should never happen in practice
			// and if so something is really wrong
			panic(fmt.Sprintf("populating duplicate block: %s", err))
		}
		// NOTE: This approach has a downside that we redo deserialization and computationally
		// expensive computation for as many duplicates. We tried solutions that doesn't have this
		// problem, but they are *much* more complex. Considering this a rare edge-case the tradeoff
		// towards simplicity has been made.
	}

	return ctx.Err()
}

// populatorFns exist to communicate between Fetch and hasher.
//
// Fetch registers PopulateFNs that hasher then uses to validate and populate Block responses coming
// through Bitswap
//
// Bitswap does not provide *stateful* verification out of the box and by default
// messages are verified by their respective MultiHashes that are registered globally.
// For every Block type there is a global hasher registered that accesses stored PopulateFn once a
// message is received. It then uses PopulateFn to validate and fill in the respective Block
//
// sync.Map is used to minimize contention for disjoint keys
var populatorFns sync.Map

// hasher implements hash.Hash to be registered as custom multihash
// hasher is the *hack* to inject custom verification logic into Bitswap
type hasher struct {
	// IDSize of the respective Shwap container
	IDSize int // to be set during hasher registration

	sum []byte
}

func (h *hasher) Write(data []byte) (int, error) {
	const pbOffset = 2 // this assumes the protobuf serialization is in use
	if len(data) < h.IDSize+pbOffset {
		err := fmt.Errorf("shwap/bitswap hasher: insufficient data size")
		log.Error()
		return 0, err
	}
	// extract Block out of data
	// we do this on the raw data to:
	//  * Avoid complicating hasher with generalized bytes -> type unmarshalling
	//  * Avoid type allocations
	id := data[pbOffset : h.IDSize+pbOffset]
	// get registered PopulateFn and use it to check data validity and
	// pass it to Fetch caller
	val, ok := populatorFns.Load(string(id))
	if !ok {
		err := fmt.Errorf("shwap/bitswap hasher: no verifier registered")
		log.Error(err)
		return 0, err
	}
	populate := val.(PopulateFn)
	err := populate(data)
	if err != nil {
		err = fmt.Errorf("shwap/bitswap hasher: verifying data: %w", err)
		log.Error(err)
		return 0, err
	}
	// set the id as resulting sum
	// it's required for the sum to match the requested ID
	// to satisfy hash contract and signal to Bitswap that data is correct
	h.sum = id
	return len(data), err
}

func (h *hasher) Sum([]byte) []byte {
	return h.sum
}

func (h *hasher) Reset() {
	h.sum = nil
}

func (h *hasher) Size() int {
	return h.IDSize
}

func (h *hasher) BlockSize() int {
	return sha256.BlockSize
}
