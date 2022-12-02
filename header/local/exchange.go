package local

import (
	"context"
	headerpkg "github.com/celestiaorg/celestia-node/pkg/header"

	"github.com/celestiaorg/celestia-node/header"
)

// NewExchange is a simple Exchange that reads Headers from Store without any networking.
type Exchange struct {
	store header.Store
}

// NewExchange creates a new local Exchange.
func NewExchange(store header.Store) header.Exchange {
	return &Exchange{
		store: store,
	}
}

func (l *Exchange) Start(context.Context) error {
	return nil
}

func (l *Exchange) Stop(context.Context) error {
	return nil
}

func (l *Exchange) Head(ctx context.Context) (*header.ExtendedHeader, error) {
	return l.store.Head(ctx)
}

func (l *Exchange) GetByHeight(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return l.store.GetByHeight(ctx, height)
}

func (l *Exchange) GetRangeByHeight(ctx context.Context, origin, amount uint64) ([]*header.ExtendedHeader, error) {
	if amount == 0 {
		return nil, nil
	}
	return l.store.GetRangeByHeight(ctx, origin, origin+amount)
}

func (l *Exchange) GetVerifiedRange(ctx context.Context, from *header.ExtendedHeader, amount uint64,
) ([]*header.ExtendedHeader, error) {
	return l.store.GetVerifiedRange(ctx, from, uint64(from.Height())+amount)
}

func (l *Exchange) Get(ctx context.Context, hash headerpkg.Hash) (*header.ExtendedHeader, error) {
	return l.store.Get(ctx, hash)
}
