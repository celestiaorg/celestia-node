package block

import (
	"context"
)

// Fetcher encompasses the behavior necessary to fetch new "raw" blocks.
type Fetcher interface {
	GetBlock(ctx context.Context, height *int64) (*RawBlock, error)
	SubscribeNewBlockEvent(ctx context.Context) (<-chan *RawBlock, error)
	UnsubscribeNewBlockEvent(ctx context.Context) error
}
