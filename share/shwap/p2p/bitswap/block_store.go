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

func (b *Blockstore) getBlockAndAccessor(ctx context.Context, cid cid.Cid) (Block, eds.AccessorStreamer, error) {
	blk, err := EmptyBlock(cid)
	if err != nil {
		return nil, nil, err
	}

	acc, err := b.Getter.GetByHeight(ctx, blk.Height())
	if errors.Is(err, store.ErrNotFound) {
		log.Debugf("no EDS Accessor for height %v found", blk.Height())
		return nil, nil, ipld.ErrNotFound{Cid: cid}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("getting EDS Accessor for height %v: %w", blk.Height(), err)
	}

	return blk, acc, nil
}

func (b *Blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	blk, acc, err := b.getBlockAndAccessor(ctx, cid)
	if err != nil {
		return nil, err
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

func (b *Blockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	// NOTE: Bitswap prioritizes peers based on their active/pending work and the priority that peers set for requests(work)
	// themselves. The prioritization happens on the Get operation of Blockstore not GetSize, while GetSize is expected
	// to be as lightweight as possible.
	//
	// Here is the best case we only open the Accessor and getting its size, avoiding expensive compute to get the size.
	blk, acc, err := b.getBlockAndAccessor(ctx, cid)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := acc.Close(); err != nil {
			log.Warnf("failed to close EDS accessor for height %v: %s", blk.Height(), err)
		}
	}()

	size, err := blk.Size(ctx, acc)
	if err != nil {
		return 0, fmt.Errorf("getting block size: %w", err)
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
