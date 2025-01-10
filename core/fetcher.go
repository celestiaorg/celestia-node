// Package core provides fundamental blockchain interaction functionality,
// including block fetching, subscription handling, and validator set management.
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

// Constants for event subscription
const newBlockSubscriber = "NewBlock/Events"

var (
	// log is the package-level logger
	log = logging.Logger("core")
	// newDataSignedBlockQuery defines the query string for subscribing to signed block events
	newDataSignedBlockQuery = types.QueryForEvent(types.EventSignedBlock).String()
)

// BlockFetcher provides functionality to fetch blocks, commits, and validator sets
// from a Tendermint Core node. It also supports subscribing to new block events.
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

// GetBlockInfo queries Core for additional block information, including Commit and ValidatorSet.
// If height is nil, it fetches information for the latest block.
// Returns an error if either the commit or validator set cannot be retrieved.
func (f *BlockFetcher) GetBlockInfo(ctx context.Context, height *int64) (*types.Commit, *types.ValidatorSet, error) {
	commit, err := f.Commit(ctx, height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commit at height %v: %w", height, err)
	}

	// If a nil height is given, we use the commit's height to ensure consistency
	// between the commit and validator set
	valSet, err := f.ValidatorSet(ctx, &commit.Height)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get validator set at height %v: %w", height, err)
	}

	return commit, valSet, nil
}

// GetBlock retrieves a block at the specified height from Core.
// If height is nil, it fetches the latest block.
func (f *BlockFetcher) GetBlock(ctx context.Context, height *int64) (*types.Block, error) {
	res, err := f.client.Block(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	if res != nil && res.Block == nil {
		return nil, fmt.Errorf("block not found at height %v", height)
	}

	return res.Block, nil
}

// GetBlockByHash retrieves a block with the specified hash from Core.
func (f *BlockFetcher) GetBlockByHash(ctx context.Context, hash libhead.Hash) (*types.Block, error) {
	res, err := f.client.BlockByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash: %w", err)
	}

	if res != nil && res.Block == nil {
		return nil, fmt.Errorf("block not found with hash %s", hash.String())
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
	// Validate height if provided
	if height != nil && *height < 0 {
		return nil, fmt.Errorf("invalid height: %d, height must be non-negative", *height)
	}

	const (
		perPage      = 100 // number of validators per page
		maxPages     = 100 // protection against too many iterations
	)

	vals := make([]*types.Validator, 0)
	total := -1
	page := 1

	for len(vals) != total {
		// Protection against too many iterations
		if page > maxPages {
			return nil, fmt.Errorf("exceeded maximum number of pages (%d) while fetching validator set", maxPages)
		}

		res, err := f.client.Validators(ctx, height, &page, &perPage)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch validators at height %v, page %d: %w", height, page, err)
		}

		if res == nil {
			return nil, fmt.Errorf("received nil response while fetching validators at height %v, page %d", height, page)
		}

		if len(res.Validators) == 0 {
			if page == 1 {
				return nil, fmt.Errorf("validator set not found at height %v", height)
			}
			// This shouldn't happen if total was correct
			return nil, fmt.Errorf("unexpected empty validator page %d when total is %d", page, total)
		}

		// Initialize total on first page
		if total == -1 {
			total = res.Total
			// Pre-allocate the slice with the total size
			vals = make([]*types.Validator, 0, total)
		}

		vals = append(vals, res.Validators...)
		page++

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while fetching validators: %w", ctx.Err())
		default:
		}
	}

	return types.NewValidatorSet(vals), nil
}

// SubscribeNewBlockEvent subscribes to new block events from Core.
// Returns a channel that receives new block events and an error if the subscription fails.
// The subscription can be cancelled using UnsubscribeNewBlockEvent or when the context is cancelled.
func (f *BlockFetcher) SubscribeNewBlockEvent(ctx context.Context) (<-chan types.EventDataSignedBlock, error) {
	if !f.client.IsRunning() {
		return nil, errors.New("client is not running")
	}

	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.doneCh = make(chan struct{})

	eventChan, err := f.client.Subscribe(ctx, newBlockSubscriber, newDataSignedBlockQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to new blocks: %w", err)
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
					log.Errorw("new blocks subscription channel closed unexpectedly")
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
