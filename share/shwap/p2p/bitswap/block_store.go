package bitswap

import (
	"context"
	"errors"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/store"
)

// AccessorGetter abstracts storage system that indexes and manages multiple eds.AccessorGetter by
// network height.
type AccessorGetter interface {
	// GetByHeight returns an Accessor by its height.
	GetByHeight(ctx context.Context, height uint64) (eds.AccessorStreamer, error)
	// HasByHeight reports whether an Accessor for the height exists.
	HasByHeight(ctx context.Context, height uint64) (bool, error)
}

// Blockstore implements generalized Bitswap compatible storage over Shwap containers
// that operates with Block and accesses data through AccessorGetter.
type Blockstore struct {
	Getter AccessorGetter
}

func (b *Blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	blk, err := EmptyBlock(cid)
	if err != nil {
		return nil, err
	}

	acc, err := b.Getter.GetByHeight(ctx, blk.Height())
	if errors.Is(err, store.ErrNotFound) {
		log.Debugf("no EDS Accessor for height %v found", blk.Height())
		return nil, ipld.ErrNotFound{Cid: cid}
	}
	if err != nil {
		return nil, fmt.Errorf("getting EDS Accessor for height %v: %w", blk.Height(), err)
	}

	defer func() {
		if err := acc.Close(); err != nil {
			log.Warnf("failed to close EDS accessor for height %v: %s", blk.Height(), err)
		}
	}()

	if err = blk.Populate(ctx, acc); err != nil {
		return nil, fmt.Errorf("failed to populate Shwap Block on height %v: %w", blk.Height(), err)
	}

	return convertBitswap(blk)
}

func (b *Blockstore) GetSize(_ context.Context, cid cid.Cid) (int, error) {
	// NOTE: Size is used as a weight for the incoming Bitswap requests. Bitswap uses fair scheduling for the requests
	// and prioritizes peers with less *active* work. Active work of a peer is a cumulative weight of all the in-progress
	// requests.

	// Constant max block size is used instead of factual size. This avoids disk IO but equalizes the weights of the
	// requests of the same type. E.g. row of 2MB EDS and row of 8MB EDS will have the same weight.
	size, err := maxBlockSize(cid)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func (b *Blockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	blk, err := EmptyBlock(cid)
	if err != nil {
		return false, err
	}

	_, err = b.Getter.HasByHeight(ctx, blk.Height())
	if err != nil {
		return false, fmt.Errorf("checking EDS Accessor for height %v: %w", blk.Height(), err)
	}
	return true, nil
}

func (b *Blockstore) Put(context.Context, blocks.Block) error {
	panic("not implemented")
}

func (b *Blockstore) PutMany(context.Context, []blocks.Block) error {
	panic("not implemented")
}

func (b *Blockstore) DeleteBlock(context.Context, cid.Cid) error {
	panic("not implemented")
}

func (b *Blockstore) AllKeysChan(context.Context) (<-chan cid.Cid, error) { panic("not implemented") }

func (b *Blockstore) HashOnRead(bool) { panic("not implemented") }

// convertBitswap converts and marshals Block to Bitswap Block.
func convertBitswap(blk Block) (blocks.Block, error) {
	protoData, err := marshalProto(blk)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap Block with proto: %w", err)
	}

	bitswapBlk, err := blocks.NewBlockWithCid(protoData, blk.CID())
	if err != nil {
		return nil, fmt.Errorf("assembling Bitswap block: %w", err)
	}

	return bitswapBlk, nil
}
