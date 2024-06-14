package bitswap

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	bitswappb "github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap/pb"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

type Accessors interface {
	Get(ctx context.Context, height uint64) (eds.Accessor, error)
}

type Blockstore struct {
	Accessors
}

func (b *Blockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	spec, ok := specRegistry[cid.Prefix().MhType]
	if !ok {
		return nil, fmt.Errorf("unsupported codec")
	}

	blk, err := spec.builder(cid)
	if err != nil {
		return nil, err
	}

	eds, err := b.Accessors.Get(ctx, blk.Height())
	if err != nil {
		return nil, err
	}

	if err = blk.Populate(ctx, eds); err != nil {
		return nil, fmt.Errorf("failed to populate Shwap Block: %w", err)
	}

	containerData, err := blk.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling Shwap container: %w", err)
	}

	blkProto := bitswappb.Block{
		Cid:       cid.Bytes(),
		Container: containerData,
	}

	blkData, err := blkProto.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling Bitswap Block protobuf: %w", err)
	}

	bitswapBlk, err := blocks.NewBlockWithCid(blkData, cid)
	if err != nil {
		return nil, fmt.Errorf("assembling Bitswap block: %w", err)
	}

	return bitswapBlk, nil
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
