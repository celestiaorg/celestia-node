package bitswap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	"github.com/celestiaorg/celestia-node/share"
	bitswappb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"
)

// WithFetcher instructs [Fetch] to use the given Fetcher.
// Useful for reusable Fetcher sessions.
func WithFetcher(session exchange.Fetcher) FetchOption {
	return func(options *fetchOptions) {
		options.Session = session
	}
}

// WithStore instructs [Fetch] to store all the fetched Blocks into the given Blockstore.
func WithStore(store blockstore.Blockstore) FetchOption {
	return func(options *fetchOptions) {
		options.Store = store
	}
}

// Fetch fetches and populates given Blocks using Fetcher wrapping Bitswap.
//
// Validates Block against the given Root and skips Blocks that are already populated.
// Gracefully synchronize identical Blocks requested simultaneously.
// Blocks until either context is canceled or all Blocks are fetched and populated.
func Fetch(ctx context.Context, exchg exchange.Interface, root *share.Root, blks []Block, opts ...FetchOption) error {
	var from, to int
	for to < len(blks) {
		from, to = to, to+maxPerFetch
		if to >= len(blks) {
			to = len(blks)
		}

		err := fetch(ctx, exchg, root, blks[from:to], opts...)
		if err != nil {
			return err
		}
	}

	return ctx.Err()
}

// maxPerFetch sets the limit for maximum items in a single fetch.
// It's a heuristic coming from Bitswap, which apparently can't process more than ~1024 in a single
// GetBlock call. Going beyond that stalls the call indefinitely.
const maxPerFetch = 1024

// fetch fetches given Blocks.
// See [Fetch] for detailed description.
func fetch(ctx context.Context, exchg exchange.Interface, root *share.Root, blks []Block, opts ...FetchOption) error {
	var options fetchOptions
	for _, opt := range opts {
		opt(&options)
	}

	fetcher := options.getFetcher(exchg)
	cids := make([]cid.Cid, 0, len(blks))
	duplicates := make(map[cid.Cid]Block)
	for _, blk := range blks {
		cid := blk.CID() // memoize CID for reuse as it ain't free
		cids = append(cids, cid)

		// store the UnmarshalFn s.t. hasher can access it
		// and fill in the Block
		unmarshalFn := blk.UnmarshalFn(root)
		_, exists := unmarshalFns.LoadOrStore(cid, &unmarshalEntry{UnmarshalFn: unmarshalFn})
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

	blkCh, err := fetcher.GetBlocks(ctx, cids)
	if err != nil {
		return fmt.Errorf("requesting Bitswap blocks: %w", err)
	}

	for bitswapBlk := range blkCh { // GetBlocks closes blkCh on ctx cancellation
		// NOTE: notification for duplicates is on purpose and to cover a flaky case
		// It's harmless in practice to do additional notifications in case of duplicates
		if err := exchg.NotifyNewBlocks(ctx, bitswapBlk); err != nil {
			log.Error("failed to notify the new Bitswap block: %s", err)
		}

		blk, ok := duplicates[bitswapBlk.Cid()]
		if ok {
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
			continue
		}
		// common case: the block was populated by the hasher
		// so store it if requested
		err := options.store(ctx, bitswapBlk)
		if err != nil {
			log.Error("failed to store the new Bitswap block: %s", err)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	for _, blk := range blks {
		if blk.IsEmpty() {
			// NOTE: This check verifies that Bitswap did it job correctly and gave us everything
			// requested. If there is still an empty block somewhere this suggests there is a bug
			// on the intersection of Bitswap and Fetch function.
			return fmt.Errorf("got empty block from Bitswap: %s", blk.CID())
		}
	}

	return nil
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
	// getBlock ID out of CID validating it
	id, err := extractCID(cid)
	if err != nil {
		return nil, fmt.Errorf("validating cid: %w", err)
	}

	if unmarshalFn == nil {
		// getBlock registered UnmarshalFn and use it to check data validity and
		// pass it to Fetch caller
		val, ok := unmarshalFns.Load(cid)
		if !ok {
			return nil, fmt.Errorf("no unmarshallers registered for %s", cid.String())
		}
		entry := val.(*unmarshalEntry)

		// ensure UnmarshalFn is synchronized
		// NOTE: Bitswap may call hasher.Write concurrently, which may call unmarshall concurrently
		// this we need this synchronization.
		entry.Lock()
		defer entry.Unlock()
		unmarshalFn = entry.UnmarshalFn
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

// unmarshalEntry wraps UnmarshalFn with a mutex to protect it from concurrent access.
type unmarshalEntry struct {
	sync.Mutex
	UnmarshalFn
}

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

type FetchOption func(*fetchOptions)

type fetchOptions struct {
	Session exchange.Fetcher
	Store   blockstore.Blockstore
}

func (options *fetchOptions) getFetcher(exhng exchange.Interface) exchange.Fetcher {
	if options.Session != nil {
		return options.Session
	}

	return exhng
}

func (options *fetchOptions) store(ctx context.Context, blk blocks.Block) error {
	if options.Store == nil {
		return nil
	}

	return options.Store.Put(ctx, blk)
}
