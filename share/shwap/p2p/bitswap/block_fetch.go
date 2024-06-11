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
	fetching := make(map[cid.Cid]Block)
	for _, blk := range blks {
		if !blk.IsEmpty() {
			continue // skip populated Blocks
		}
		// memoize CID for reuse as it ain't free
		cid := blk.CID()
		fetching[cid] = blk // mark block as fetching
		cids = append(cids, cid)
		// store the PopulateFn s.t. hasher can access it
		// and fill in the Block
		populate := blk.PopulateFn(root)
		populatorFns.LoadOrStore(cid, populate)
	}

	blkCh, err := fetcher.GetBlocks(ctx, cids)
	if err != nil {
		return fmt.Errorf("fetching bitswap blocks: %w", err)
	}

	for bitswapBlk := range blkCh { // GetBlocks closes blkCh on ctx cancellation
		blk := fetching[bitswapBlk.Cid()]
		if !blk.IsEmpty() {
			// common case: the block was populated by the hasher, so skip
			continue
		}
		// uncommon duplicate case: concurrent fetching of the same block,
		// so we have to populate it ourselves instead of the hasher,
		err := blk.PopulateFn(root)(bitswapBlk.RawData())
		if err != nil {
			// this means verification succeeded in the hasher but failed here
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
	if len(data) == 0 {
		errMsg := "hasher: empty message"
		log.Error(errMsg)
		return 0, fmt.Errorf("shwap/bitswap: %s", errMsg)
	}

	// cut off the first tag type byte out of protobuf data
	const pbTypeOffset = 1
	cidData := data[pbTypeOffset:]

	cid, err := readCID(cidData)
	if err != nil {
		err = fmt.Errorf("hasher: reading cid: %w", err)
		log.Error(err)
		return 0, fmt.Errorf("shwap/bitswap: %w", err)
	}
	// get ID out of CID and validate it
	id, err := extractCID(cid)
	if err != nil {
		err = fmt.Errorf("hasher: validating cid: %w", err)
		log.Error(err)
		return 0, fmt.Errorf("shwap/bitswap: %w", err)
	}
	// get registered PopulateFn and use it to check data validity and
	// pass it to Fetch caller
	val, ok := populatorFns.LoadAndDelete(cid)
	if !ok {
		errMsg := "hasher: no verifier registered"
		log.Error(errMsg)
		return 0, fmt.Errorf("shwap/bitswap: %s", errMsg)
	}
	populate := val.(PopulateFn)
	err = populate(data)
	if err != nil {
		err = fmt.Errorf("hasher: verifying data: %w", err)
		log.Error(err)
		return 0, fmt.Errorf("shwap/bitswap: %w", err)
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
