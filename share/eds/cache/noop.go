package cache

import (
	"context"
	"io"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
)

var _ Cache = (*NoopCache)(nil)

// NoopCache implements noop version of Cache interface
type NoopCache struct{}

func (n NoopCache) Get(shard.Key) (Accessor, error) {
	return nil, errCacheMiss
}

func (n NoopCache) GetOrLoad(
	context.Context, shard.Key,
	func(context.Context, shard.Key) (Accessor, error),
) (Accessor, error) {
	return NoopAccessor{}, nil
}

func (n NoopCache) Remove(shard.Key) error {
	return nil
}

func (n NoopCache) EnableMetrics() error {
	return nil
}

var _ Accessor = (*NoopAccessor)(nil)

// NoopAccessor implements noop version of Accessor interface
type NoopAccessor struct{}

func (n NoopAccessor) Blockstore() (dagstore.ReadBlockstore, error) {
	return nil, nil
}

func (n NoopAccessor) Reader() io.Reader {
	return nil
}

func (n NoopAccessor) Close() error {
	return nil
}
