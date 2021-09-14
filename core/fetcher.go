package core

import (
	"context"
	"fmt"

	rpctypes "github.com/celestiaorg/celestia-core/rpc/core/types"
	"github.com/celestiaorg/celestia-core/types"
	"github.com/celestiaorg/celestia-node/service/block"
)

const newBlockSubscriber = "NewBlock/Events"

var newBlockEventQuery = types.QueryForEvent(types.EventNewBlock).String()

type BlockFetcher struct {
	Client

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

	// create a wrapper channel for translating ResultEvent to "raw" block
	if f.newBlockCh != nil {
		return nil, fmt.Errorf("new block event channel exists")
	}
	newBlockChan := make(chan *block.Raw)
	f.newBlockCh = newBlockChan

	go func(eventChan <-chan rpctypes.ResultEvent, newBlockChan chan *block.Raw) {
		for {
			newEvent := <-eventChan
			rawBlock, ok := newEvent.Data.(types.EventDataNewBlock)
			if !ok {
				// TODO log & ignore
				continue
			}
			newBlockChan <- rawBlock.Block
		}
	}(eventChan, newBlockChan)

	return newBlockChan, nil
}

// UnsubscribeNewBlockEvent stops the subscription to new block events from Core.
func (f *BlockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	// send done signal
	ctx.Done()
	// close the new block channel
	if f.newBlockCh == nil {
		return fmt.Errorf("no new block event channel found")
	}
	close(f.newBlockCh)
	f.newBlockCh = nil

	return f.Unsubscribe(ctx, newBlockSubscriber, newBlockEventQuery)
}
