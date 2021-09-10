package core

import (
	"context"
	"sync"

	rpctypes "github.com/celestiaorg/celestia-core/rpc/core/types"
	"github.com/celestiaorg/celestia-core/types"
	"github.com/celestiaorg/celestia-node/service/block"
)

const newBlockSubscriber = "NewBlock/Events"

var newBlockEventQuery = types.QueryForEvent(types.EventNewBlock).String()

type BlockFetcher struct {
	Client

	mux        sync.Mutex
	newBlockCh chan *block.Raw
}

// NewBlockFetcher returns a new `BlockFetcher`.
func NewBlockFetcher(client Client) *BlockFetcher {
	return &BlockFetcher{
		Client: client,
	}
}

// GetBlock queries Core for a `Block` at the given height.
func (f *BlockFetcher) GetBlock(ctx context.Context, height *int64) (*block.Raw, error) {
	raw, err := f.Block(ctx, height)
	return raw.Block, err
}

// SubscribeNewBlockEvent subscribes to new block events from Core, returning
// a new block event channel on success.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan *block.Raw, error) {
	// start the client if not started yet
	if !f.IsRunning() {
		if err := f.Start(); err != nil {
			return nil, err
		}
	}
	eventChan, err := f.Subscribe(ctx, newBlockSubscriber, newBlockEventQuery)
	if err != nil {
		return nil, err
	}

	// create a wrapper channel for translating ResultEvent to *core.Block
	newBlockChan := make(chan *block.Raw)
	f.mux.Lock()
	f.newBlockCh = newBlockChan
	f.mux.Unlock()

	go func(eventChan <-chan rpctypes.ResultEvent, newBlockChan chan *block.Raw) {
		for {
			newEvent := <-eventChan
			rawBlock, ok := newEvent.Data.(types.EventDataNewBlock)
			if !ok {
				// TODO log & ignore? or errChan?
				continue
			}
			newBlockChan <- rawBlock.Block
		}
	}(eventChan, newBlockChan)

	return newBlockChan, nil
}

// UnsubscribeNewBlockEvent stops the subscription to new block events from Core.
func (f *BlockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	// close the new block channel
	close(f.newBlockCh)
	f.mux.Lock()
	f.newBlockCh = nil
	f.mux.Unlock()

	return f.Unsubscribe(ctx, newBlockSubscriber, newBlockEventQuery)
}
