package cache

import (
	"context"

	"github.com/filecoin-project/dagstore/shard"
)

var _ Cache = (*NoopCache)(nil)

// NoopCache implements noop version of Cache interface
type NoopCache struct{}

func (n NoopCache) Get(shard.Key) (Accessor, error) {
	return nil, ErrCacheMiss
}

func (n NoopCache) GetOrLoad(
	context.Context, shard.Key,
	func(context.Context, shard.Key) (Accessor, error),
) (Accessor, error) {
	return nil, nil
}

func (n NoopCache) Remove(shard.Key) error {
	return nil
}

func (n NoopCache) EnableMetrics() error {
	return nil
}
