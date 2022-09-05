package edsstore

import (
	"context"
	"fmt"

	blocks "github.com/ipfs/go-block-format"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-verifcid"
)

// CARNodeAdder is needed to fulfill the NodeAdder interface
type CARNodeAdder struct {
	exchange   exchange.Interface
	blockstore blockstore.Blockstore
}

func New(bs blockstore.Blockstore, ex exchange.Interface) *CARNodeAdder {
	return &CARNodeAdder{
		exchange:   ex,
		blockstore: bs,
	}
}

func (na *CARNodeAdder) Add(ctx context.Context, nd format.Node) error {
	if na == nil { // FIXME remove this assertion. protect with constructor invariant
		return fmt.Errorf("dagService is nil")
	}
	return na.AddBlock(ctx, nd)
}

func (na *CARNodeAdder) AddMany(ctx context.Context, nds []format.Node) error {
	blks := make([]blocks.Block, len(nds))
	for i, nd := range nds {
		blks[i] = nd
	}
	return na.AddBlocks(ctx, blks)
}

// AddBlock adds a particular block to the service, Putting it into the datastore.
func (na *CARNodeAdder) AddBlock(ctx context.Context, o blocks.Block) error {
	c := o.Cid()
	// hash security
	err := verifcid.ValidateCid(c)
	if err != nil {
		return err
	}

	if err := na.blockstore.Put(ctx, o); err != nil {
		return err
	}

	if err := na.exchange.HasBlock(ctx, o); err != nil {
		panic("Couldnt add block")
	}

	return nil
}

func (na *CARNodeAdder) AddBlocks(ctx context.Context, blks []blocks.Block) error {
	// hash security
	for _, b := range blks {
		err := verifcid.ValidateCid(b.Cid())
		if err != nil {
			return err
		}
	}

	toput := blks

	if len(toput) == 0 {
		return nil
	}

	err := na.blockstore.PutMany(ctx, toput)
	if err != nil {
		edsStoreLog.Warn("failed to put blocks in CAR: ", err)
		return err
	}

	for _, o := range toput {
		if err := na.exchange.HasBlock(ctx, o); err != nil {
			panic("Couldnt add block")
		}
	}
	return nil
}
