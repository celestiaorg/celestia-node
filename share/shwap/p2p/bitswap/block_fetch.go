package bitswap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/exchange"
	"github.com/ipfs/go-cid"

	bitswappb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"

	"github.com/celestiaorg/celestia-node/share"
)

// Fetch fetches and populates given Blocks using Fetcher wrapping Bitswap.
//
// Validates Block against the given Root and skips Blocks that are already populated.
// Gracefully synchronize identical Blocks requested simultaneously.
// Blocks until either context is canceled or all Blocks are fetched and populated.
func Fetch(ctx context.Context, exchg exchange.Interface, root *share.Root, blks ...Block) error {
	cids := make([]cid.Cid, 0, len(blks))
	duplicates := make(map[cid.Cid]Block)
	for _, blk := range blks {
		if !blk.IsEmpty() {
			continue // skip populated Blocks
		}

		cid := blk.CID() // memoize CID for reuse as it ain't free
		cids = append(cids, cid)

		// store the UnmarshalFn s.t. hasher can access it
		// and fill in the Block
		unmarshalFn := blk.UnmarshalFn(root)
		_, exists := unmarshalFns.LoadOrStore(cid, unmarshalFn)
		if exists {
			// the unmarshalFn has already been stored for the cid
			// means there is ongoing fetch happening for the same cid
			duplicates[cid] = blk // so mark the Block as duplicate
		} else {
			// cleanup are by the original requester and
			// only after we are sure we got the block
			defer unmarshalFns.Delete(cid)
		}
	}

	blkCh, err := exchg.GetBlocks(ctx, cids)
	if err != nil {
		return fmt.Errorf("requesting Bitswap blocks: %w", err)
	}

	for bitswapBlk := range blkCh { // GetBlocks closes blkCh on ctx cancellation
		if err := exchg.NotifyNewBlocks(ctx, bitswapBlk); err != nil {
			log.Error("failed to notify new Bitswap blocks: %w", err)
		}

		blk, ok := duplicates[bitswapBlk.Cid()]
		if !ok {
			// common case: the block was populated by the hasher, so skip
			continue
		}
		// uncommon duplicate case: concurrent fetching of the same block,
		// so we have to unmarshal it ourselves instead of the hasher,
		unmarshalFn := blk.UnmarshalFn(root)
		_, err := unmarshal(unmarshalFn, bitswapBlk.RawData())
		if err != nil {
			// this means verification succeeded in the hasher but failed here
			// this case should never happen in practice
			// and if so something is really wrong
			panic(fmt.Sprintf("unmarshaling duplicate block: %s", err))
		}
		// NOTE: This approach has a downside that we redo deserialization and computationally
		// expensive computation for as many duplicates. We tried solutions that doesn't have this
		// problem, but they are *much* more complex. Considering this a rare edge-case the tradeoff
		// towards simplicity has been made.
	}

	return ctx.Err()
}

// unmarshal unmarshalls the Shwap Container data into a Block via UnmarshalFn
// If unmarshalFn is nil -- gets it from the global unmarshalFns.
func unmarshal(unmarshalFn UnmarshalFn, data []byte) ([]byte, error) {
	var blk bitswappb.Block
	err := blk.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling block: %w", err)
	}
	cid, err := cid.Cast(blk.Cid)
	if err != nil {
		return nil, fmt.Errorf("casting cid: %w", err)
	}
	// get ID out of CID validating it
	id, err := extractCID(cid)
	if err != nil {
		return nil, fmt.Errorf("validating cid: %w", err)
	}

	if unmarshalFn == nil {
		// get registered UnmarshalFn and use it to check data validity and
		// pass it to Fetch caller
		val, ok := unmarshalFns.Load(cid)
		if !ok {
			return nil, fmt.Errorf("no unmarshallers registered for %s", cid.String())
		}
		unmarshalFn = val.(UnmarshalFn)
	}

	err = unmarshalFn(blk.Container)
	if err != nil {
		return nil, fmt.Errorf("verifying data: %w", err)
	}

	return id, nil
}

// unmarshalFns exist to communicate between Fetch and hasher, and it's global as a necessity
//
// Fetch registers UnmarshalFNs that hasher then uses to validate and unmarshal Block responses coming
// through Bitswap
//
// Bitswap does not provide *stateful* verification out of the box and by default
// messages are verified by their respective MultiHashes that are registered globally.
// For every Block type there is a global hasher registered that accesses stored UnmarshalFn once a
// message is received. It then uses UnmarshalFn to validate and fill in the respective Block
//
// sync.Map is used to minimize contention for disjoint keys
var unmarshalFns sync.Map

// hasher implements hash.Hash to be registered as custom multihash
// hasher is the *hack* to inject custom verification logic into Bitswap
type hasher struct {
	// IDSize of the respective Shwap container
	IDSize int // to be set during hasher registration

	sum []byte
}

func (h *hasher) Write(data []byte) (int, error) {
	id, err := unmarshal(nil, data)
	if err != nil {
		err = fmt.Errorf("hasher: %w", err)
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
