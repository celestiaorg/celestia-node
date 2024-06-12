package bitswap

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

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

	return blk.BlockFromEDS(ctx, eds)
}

func (b *Blockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
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
