package block

import (
	"context"
)

// Fetcher encompasses the behavior necessary to fetch new blocks.
type Fetcher interface {
	GetBlock(ctx context.Context, height *int64) (*Raw, error)
	SubscribeNewBlockEvent(ctx context.Context) (<-chan *Raw, error)
	UnsubscribeNewBlockEvent(ctx context.Context) error
}
