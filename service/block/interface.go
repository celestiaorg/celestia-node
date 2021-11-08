package block

import (
	"context"

	core "github.com/tendermint/tendermint/types"
)

// Fetcher encompasses the behavior necessary to fetch new "raw" blocks.
type Fetcher interface {
	GetBlock(ctx context.Context, height *int64) (*RawBlock, error)
	Commit(ctx context.Context, height *int64) (*core.Commit, error)
	ValidatorSet(ctx context.Context, height *int64) (*core.ValidatorSet, error)
	SubscribeNewBlockEvent(ctx context.Context) (<-chan *RawBlock, error)
	UnsubscribeNewBlockEvent(ctx context.Context) error
}
