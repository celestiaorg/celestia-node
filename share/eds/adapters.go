package eds

import (
	"context"
	"sync"

	"github.com/filecoin-project/dagstore"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/blocks"
)

var _ blockservice.BlockGetter = (*BlockGetter)(nil)

// NewBlockGetter creates new blockservice.BlockGetter adapter from dagstore.ReadBlockstore
func NewBlockGetter(store dagstore.ReadBlockstore) *BlockGetter {
	return &BlockGetter{store: store}
}

// BlockGetter is an adapter for dagstore.ReadBlockstore to implement blockservice.BlockGetter
// interface.
type BlockGetter struct {
	store dagstore.ReadBlockstore
}

// GetBlock gets the requested block by the given CID.
func (bg *BlockGetter) GetBlock(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	return bg.store.Get(ctx, cid)
}

// GetBlocks does a batch request for the given cids, returning blocks as
// they are found, in no particular order.
//
// It implements blockservice.BlockGetter interface, that requires:
// It may not be able to find all requested blocks (or the context may
// be canceled). In that case, it will close the channel early. It is up
// to the consumer to detect this situation and keep track which blocks
// it has received and which it hasn't.
func (bg *BlockGetter) GetBlocks(ctx context.Context, cids []cid.Cid) <-chan blocks.Block {
	bCh := make(chan blocks.Block)

	go func() {
		var wg sync.WaitGroup
		wg.Add(len(cids))
		for _, c := range cids {
			go func(cid cid.Cid) {
				defer wg.Done()
				block, err := bg.store.Get(ctx, cid)
				if err != nil {
					log.Debugw("getblocks: error getting block by cid", "cid", cid, "error", err)
					return
				}

				select {
				case bCh <- block:
				case <-ctx.Done():
					return
				}
			}(c)
		}
		wg.Wait()
		close(bCh)
	}()

	return bCh
}
