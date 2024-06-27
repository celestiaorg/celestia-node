package bitswap

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

// AccessorGetter abstracts storage system that indexes and manages multiple eds.AccessorGetter by network height.
type AccessorGetter interface {
	// GetByHeight returns an Accessor by its height.
	GetByHeight(ctx context.Context, height uint64) (eds.AccessorCloser, error)
}

// Blockstore implements generalized Bitswap compatible storage over Shwap containers
// that operates with Block and accesses data through AccessorGetter.
type Blockstore struct {
	Getter AccessorGetter
}

func (b *Blockstore) getBlock(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	spec, ok := specRegistry[cid.Prefix().MhType]
	if !ok {
		return nil, fmt.Errorf("unsupported Block type: %v", cid.Prefix().MhType)
	}

	blk, err := spec.builder(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to build a Block for %s: %w", spec.String(), err)
	}

	eds, err := b.Getter.GetByHeight(ctx, blk.Height())
	if err != nil {
		return nil, fmt.Errorf("getting EDS Accessor for height %v: %w", blk.Height(), err)
	}
	defer func() {
		if err := eds.Close(); err != nil {
			log.Warnf("failed to close EDS accessor for height %v: %s", blk.Height(), err)
		}
	}()

	if err = blk.Populate(ctx, eds); err != nil {
		return nil, fmt.Errorf("failed to populate Shwap Block on height %v for %s: %w", blk.Height(), spec.String(), err)
	}

	return convertBitswap(blk)
}

func (b *Blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	blk, err := b.getBlock(ctx, cid)
	if err != nil {
		log.Errorf("blockstore: getting local block(%s): %s", cid, err)
		return nil, err
	}

	return blk, nil
}

func (b *Blockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	// TODO(@Wondertan): There must be a way to derive size without reading, proving, serializing and
	//  allocating Sample's block.Block or we could do hashing
	// NOTE:Bitswap uses GetSize also to determine if we have content stored or not
	// so simply returning constant size is not an option
	blk, err := b.Get(ctx, cid)
	if err != nil {
		return 0, err
	}
	return len(blk.RawData()), nil
}

func (b *Blockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	_, err := b.Get(ctx, cid)
	if err != nil {
		return false, err
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
