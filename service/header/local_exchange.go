package header

import "context"

type LocalExchange struct {
	store Store
}

func NewLocalExchange(store Store) Exchange {
	return &LocalExchange{
		store: store,
	}
}

func (l *LocalExchange) RequestHead(ctx context.Context) (*ExtendedHeader, error) {
	return l.store.Head(ctx)
}

func (l *LocalExchange) RequestHeader(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	return l.store.GetByHeight(ctx, height)
}

func (l *LocalExchange) RequestHeaders(ctx context.Context, origin, amount uint64) ([]*ExtendedHeader, error) {
	return l.store.GetRangeByHeight(ctx, origin, origin+amount)
}

func (l *LocalExchange) RequestByHash(ctx context.Context, hash []byte) (*ExtendedHeader, error) {
	return l.store.Get(ctx, hash)
}
