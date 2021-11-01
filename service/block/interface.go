package block

import (
	"context"

	core "github.com/celestiaorg/celestia-core/types"
)

// Fetcher encompasses the behavior necessary to fetch new "raw" blocks.
type Fetcher interface {
	GetBlock(ctx context.Context, height *int64) (*RawBlock, error)
	CommitAtHeight(ctx context.Context, height *int64) (*core.Commit, error)
	SubscribeNewBlockEvent(ctx context.Context) (<-chan *RawBlock, error)
	UnsubscribeNewBlockEvent(ctx context.Context) error
}
