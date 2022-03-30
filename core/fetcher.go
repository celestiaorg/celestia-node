package core

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

const newBlockSubscriber = "NewBlock/Events"

var (
	log                = logging.Logger("core/fetcher")
	newBlockEventQuery = types.QueryForEvent(types.EventNewBlock).String()
)

type BlockFetcher struct {
	client Client

	newBlockCh chan *types.Block
	doneCh     chan struct{}
}

// NewBlockFetcher returns a new `BlockFetcher`.
func NewBlockFetcher(client Client) *BlockFetcher {
	return &BlockFetcher{
		client: client,
	}
}

// GetBlockInfo queries Core for additional block information, like Commit and ValidatorSet.
func (f *BlockFetcher) GetBlockInfo(ctx context.Context, height *int64) (*types.Commit, *types.ValidatorSet, error) {
	commit, err := f.Commit(ctx, height)
	if err != nil {
		return nil, nil, fmt.Errorf("core/fetcher: getting commit: %w", err)
	}

	// If a nil `height` is given as a parameter, there is a chance
	// that a new block could be produced between getting the latest
	// commit and getting the latest validator set. Therefore, it is
	// best to get the validator set at the latest commit's height to
	// prevent this potential inconsistency.
	valSet, err := f.ValidatorSet(ctx, &commit.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("core/fetcher: getting validator set: %w", err)
	}

	return commit, valSet, nil
}

// GetBlock queries Core for a `Block` at the given height.
func (f *BlockFetcher) GetBlock(ctx context.Context, height *int64) (*types.Block, error) {
	res, err := f.client.Block(ctx, height)
	if err != nil {
		return nil, err
	}

	if res != nil && res.Block == nil {
		return nil, fmt.Errorf("core/fetcher: block not found")
	}

	return res.Block, nil
}

func (f *BlockFetcher) GetBlockByHash(ctx context.Context, hash tmbytes.HexBytes) (*types.Block, error) {
	res, err := f.client.BlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if res != nil && res.Block == nil {
		return nil, fmt.Errorf("core/fetcher: block not found")
	}

	return res.Block, nil
}

// Commit queries Core for a `Commit` from the block at
// the given height.
func (f *BlockFetcher) Commit(ctx context.Context, height *int64) (*types.Commit, error) {
	res, err := f.client.Commit(ctx, height)
	if err != nil {
		return nil, err
	}

	if res != nil && res.Commit == nil {
		return nil, fmt.Errorf("core/fetcher: commit not found")
	}

	return res.Commit, nil
}

// ValidatorSet queries Core for the ValidatorSet from the
// block at the given height.
func (f *BlockFetcher) ValidatorSet(ctx context.Context, height *int64) (*types.ValidatorSet, error) {
	var perPage = 100

	vals, total := make([]*types.Validator, 0), -1
	for page := 1; len(vals) != total; page++ {
		res, err := f.client.Validators(ctx, height, &page, &perPage)
		if err != nil {
			return nil, err
		}

		if res != nil && len(res.Validators) == 0 {
			return nil, fmt.Errorf("core/fetcher: validators not found")
		}

		total = res.Total
		vals = append(vals, res.Validators...)
	}

	return types.NewValidatorSet(vals), nil
}

// SubscribeNewBlockEvent subscribes to new block events from Core, returning
// a new block event channel on success.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan *types.Block, error) {
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

	f.newBlockCh = make(chan *types.Block)
	f.doneCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-f.doneCh:
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
				case <-f.doneCh:
					return
				}
			}
		}
	}()

	return f.newBlockCh, nil
}

// UnsubscribeNewBlockEvent stops the subscription to new block events from Core.
func (f *BlockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	if f.newBlockCh == nil {
		return fmt.Errorf("no new block event channel found")
	}
	if f.doneCh == nil {
		return fmt.Errorf("no stop signal chan found in fetcher")
	}
	defer func() {
		// send stop signal
		f.doneCh <- struct{}{}
		// close out fetcher channels
		close(f.newBlockCh)
		close(f.doneCh)
		f.newBlockCh = nil
		f.doneCh = nil
	}()

	return f.client.Unsubscribe(ctx, newBlockSubscriber, newBlockEventQuery)
}

// IsSyncing returns the sync status of the Core connection: true for
// syncing, and false for already caught up. It can also return an error
// in the case of a failed status request.
func (f *BlockFetcher) IsSyncing(ctx context.Context) (bool, error) {
	resp, err := f.client.Status(ctx)
	if err != nil {
		return false, err
	}
	return resp.SyncInfo.CatchingUp, nil
}
