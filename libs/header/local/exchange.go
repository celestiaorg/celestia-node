package local

import (
	"context"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// Exchange is a simple Exchange that reads Headers from Store without any networking.
type Exchange[H header.Header] struct {
	store header.Store[H]
}

// NewExchange creates a new local Exchange.
func NewExchange[H header.Header](store header.Store[H]) header.Exchange[H] {
	return &Exchange[H]{
		store: store,
	}
}

func (l *Exchange[H]) Start(context.Context) error {
	return nil
}

func (l *Exchange[H]) Stop(context.Context) error {
	return nil
}

func (l *Exchange[H]) Head(ctx context.Context) (H, error) {
	return l.store.Head(ctx)
}

func (l *Exchange[H]) GetByHeight(ctx context.Context, height uint64) (H, error) {
	return l.store.GetByHeight(ctx, height)
}

func (l *Exchange[H]) GetRangeByHeight(ctx context.Context, origin, amount uint64) ([]H, error) {
	if amount == 0 {
		return nil, nil
	}
	return l.store.GetRangeByHeight(ctx, origin, origin+amount)
}

func (l *Exchange[H]) GetVerifiedRange(ctx context.Context, from H, amount uint64,
) ([]H, error) {
	return l.store.GetVerifiedRange(ctx, from, uint64(from.Height())+amount)
}

func (l *Exchange[H]) Get(ctx context.Context, hash header.Hash) (H, error) {
	return l.store.Get(ctx, hash)
}
