package headerexchange

import (
	"context"

	"github.com/tendermint/tendermint/libs/bytes"

	"github.com/celestiaorg/celestia-node/header"
)

// LocalExchange is a simple Exchange that reads Headers from Store without any networking.
type LocalExchange struct {
	store header.Store
}

// NewLocalExchange creates new Exchange.
func NewLocalExchange(store header.Store) header.Exchange {
	return &LocalExchange{
		store: store,
	}
}

func (l *LocalExchange) Start(context.Context) error {
	return nil
}

func (l *LocalExchange) Stop(context.Context) error {
	return nil
}

func (l *LocalExchange) RequestHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return l.store.Head(ctx)
}

func (l *LocalExchange) RequestHeader(ctx context.Context, height uint64) (*header.ExtendedHeader, error) {
	return l.store.GetByHeight(ctx, height)
}

func (l *LocalExchange) RequestHeaders(ctx context.Context, origin, amount uint64) ([]*header.ExtendedHeader, error) {
	if amount == 0 {
		return nil, nil
	}
	return l.store.GetRangeByHeight(ctx, origin, origin+amount)
}

func (l *LocalExchange) RequestByHash(ctx context.Context, hash bytes.HexBytes) (*header.ExtendedHeader, error) {
	return l.store.Get(ctx, hash)
}
