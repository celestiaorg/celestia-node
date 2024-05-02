package core

import (
	"context"
	"errors"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"

	libhead "github.com/celestiaorg/go-header"
)

const newBlockSubscriber = "NewBlock/Events"

var (
	log                     = logging.Logger("core")
	newDataSignedBlockQuery = types.QueryForEvent(types.EventSignedBlock).String()
)

type BlockFetcher struct {
	client Client

	doneCh chan struct{}
	cancel context.CancelFunc
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
		return nil, nil, fmt.Errorf("core/fetcher: getting commit at height %d: %w", height, err)
	}

	// If a nil `height` is given as a parameter, there is a chance
	// that a new block could be produced between getting the latest
	// commit and getting the latest validator set. Therefore, it is
	// best to get the validator set at the latest commit's height to
	// prevent this potential inconsistency.
	valSet, err := f.ValidatorSet(ctx, &commit.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("core/fetcher: getting validator set at height %d: %w", height, err)
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
		return nil, fmt.Errorf("core/fetcher: block not found, height: %d", height)
	}

	return res.Block, nil
}

func (f *BlockFetcher) GetBlockByHash(ctx context.Context, hash libhead.Hash) (*types.Block, error) {
	res, err := f.client.BlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if res != nil && res.Block == nil {
		return nil, fmt.Errorf("core/fetcher: block not found, hash: %s", hash.String())
	}

	return res.Block, nil
}

// GetSignedBlock queries Core for a `Block` at the given height.
func (f *BlockFetcher) GetSignedBlock(ctx context.Context, height *int64) (*coretypes.ResultSignedBlock, error) {
	return f.client.SignedBlock(ctx, height)
}

// Commit queries Core for a `Commit` from the block at
// the given height.
func (f *BlockFetcher) Commit(ctx context.Context, height *int64) (*types.Commit, error) {
	res, err := f.client.Commit(ctx, height)
	if err != nil {
		return nil, err
	}

	if res != nil && res.Commit == nil {
		return nil, fmt.Errorf("core/fetcher: commit not found at height %d", height)
	}

	return res.Commit, nil
}

// ValidatorSet queries Core for the ValidatorSet from the
// block at the given height.
func (f *BlockFetcher) ValidatorSet(ctx context.Context, height *int64) (*types.ValidatorSet, error) {
	perPage := 100

	vals, total := make([]*types.Validator, 0), -1
	for page := 1; len(vals) != total; page++ {
		res, err := f.client.Validators(ctx, height, &page, &perPage)
		if err != nil {
			return nil, err
		}

		if res != nil && len(res.Validators) == 0 {
			return nil, fmt.Errorf("core/fetcher: validator set not found at height %d", height)
		}

		total = res.Total
		vals = append(vals, res.Validators...)
	}

	return types.NewValidatorSet(vals), nil
}

// SubscribeNewBlockEvent subscribes to new block events from Core, returning
// a new block event channel on success.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan types.EventDataSignedBlock, error) {
	// start the client if not started yet
	if !f.client.IsRunning() {
		return nil, errors.New("client not running")
	}

	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.doneCh = make(chan struct{})

	eventChan, err := f.client.Subscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery)
	if err != nil {
		return nil, err
	}

	signedBlockCh := make(chan types.EventDataSignedBlock)
	go func() {
		defer close(f.doneCh)
		defer close(signedBlockCh)
		for {
			select {
			case <-ctx.Done():
				return
			case newEvent, ok := <-eventChan:
				if !ok {
					log.Errorw("fetcher: new blocks subscription channel closed unexpectedly")
					return
				}
				signedBlock := newEvent.Data.(types.EventDataSignedBlock)
				select {
				case signedBlockCh <- signedBlock:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return signedBlockCh, nil
}

// UnsubscribeNewBlockEvent stops the subscription to new block events from Core.
func (f *BlockFetcher) UnsubscribeNewBlockEvent(ctx context.Context) error {
	f.cancel()
	select {
	case <-f.doneCh:
	case <-ctx.Done():
		return fmt.Errorf("fetcher: unsubscribe from new block events: %w", ctx.Err())
	}
	return f.client.Unsubscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery)
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
