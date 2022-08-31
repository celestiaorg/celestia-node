package dagblockstore

import (
	"context"
	"fmt"
	blocks "github.com/ipfs/go-block-format"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-verifcid"
)

type CARBlockStore struct {
	exchange   exchange.Interface
	blockstore blockstore.Blockstore
}

func New(bs blockstore.Blockstore, ex exchange.Interface) *CARBlockStore {
	return &CARBlockStore{
		exchange:   ex,
		blockstore: bs,
	}
}

func (bs *CARBlockStore) Add(ctx context.Context, nd format.Node) error {
	if bs == nil { // FIXME remove this assertion. protect with constructor invariant
		return fmt.Errorf("dagService is nil")
	}

	return bs.AddBlock(ctx, nd)
}

func (bs *CARBlockStore) AddMany(ctx context.Context, nds []format.Node) error {
	blks := make([]blocks.Block, len(nds))
	for i, nd := range nds {
		blks[i] = nd
	}
	return bs.AddBlocks(ctx, blks)
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
// TODO pass a context into this if the remote.HasBlock is going to remain here.
func (bs *CARBlockStore) AddBlock(ctx context.Context, o blocks.Block) error {
	c := o.Cid()
	// hash security
	err := verifcid.ValidateCid(c)
	if err != nil {
		return err
	}

	if err := bs.blockstore.Put(ctx, o); err != nil {
		return err
	}

	if err := bs.exchange.HasBlock(ctx, o); err != nil {
		panic("Couldnt add block")
	}

	return nil
}

func (bs *CARBlockStore) AddBlocks(ctx context.Context, blks []blocks.Block) error {
	// hash security
	for _, b := range blks {
		err := verifcid.ValidateCid(b.Cid())
		if err != nil {
			return err
		}
	}

	var toput []blocks.Block
	toput = blks

	if len(toput) == 0 {
		return nil
	}

	err := bs.blockstore.PutMany(ctx, toput)
	if err != nil {
		return err
	}

	for _, o := range toput {
		if err := bs.exchange.HasBlock(ctx, o); err != nil {
			panic("Couldnt add block")
		}
	}
	return nil
}
