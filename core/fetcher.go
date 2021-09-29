package core

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-core/types"

	"github.com/celestiaorg/celestia-node/service/block"
)

const newBlockSubscriber = "NewBlock/Events"

var newBlockEventQuery = types.QueryForEvent(types.EventNewBlock).String()

type BlockFetcher struct {
	client Client

	newBlockCh chan *block.RawBlock
}

// NewBlockFetcher returns a new `BlockFetcher`.
func NewBlockFetcher(client Client) *BlockFetcher {
	return &BlockFetcher{
		client: client,
	}
}

// GetBlock queries Core for a `Block` at the given height.
func (f *BlockFetcher) GetBlock(ctx context.Context, height *int64) (*block.RawBlock, error) {
	raw, err := f.client.Block(ctx, height)
	if err != nil {
		return nil, err
	}
	return raw.Block, nil
}

// SubscribeNewBlockEvent subscribes to new block events from Core, returning
// a new block event channel on success.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan *block.RawBlock, error) {
	// start the client if not started yet
	if !f.client.IsRunning() {
		return nil, fmt.Errorf("client not running")
	}
	eventChan, err := f.client.Subscribe(ctx, newBlockSubscriber, newBlockEventQuery)
	if err != nil {
		return nil, err
	}

	// create a wrapper channel for translating ResultEvent to "raw" block
	if f.newBlockCh != nil {
		return nil, fmt.Errorf("new block event channel exists")
	}

	f.newBlockCh = make(chan *block.RawBlock)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case newEvent, ok := <-eventChan:
				if !ok {
					return
				}
				newBlock, ok := newEvent.Data.(types.EventDataNewBlock)
				if !ok {
					log.Warnf("unexpected event: %v", newEvent)
					continue
				}
				select {
				case f.newBlockCh <- newBlock.Block:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return f.newBlockCh, nil
}

// UnsubscribeNewBlockEvent stops the subscription to new block events from Core.
func (f *BlockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	// close the new block channel
	if f.newBlockCh == nil {
		return fmt.Errorf("no new block event channel found")
	}
	defer func() {
		close(f.newBlockCh)
		f.newBlockCh = nil
	}()

	return f.client.Unsubscribe(ctx, newBlockSubscriber, newBlockEventQuery)
}
