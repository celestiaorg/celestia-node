package store

import (
	"fmt"
)

// Option is the functional option that is applied to the store instance
// to configure store parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the store.
type Parameters struct {
	// StoreCacheSize defines the maximum amount of entries in the Header Store cache.
	StoreCacheSize int

	// IndexCacheSize defines the maximum amount of entries in the Height to Hash index cache.
	IndexCacheSize int

	// WriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	WriteBatchSize int
}

// DefaultParameters returns the default params to configure the store.
func DefaultParameters() Parameters {
	return Parameters{
		StoreCacheSize: 4096,
		IndexCacheSize: 16384,
		WriteBatchSize: 2048,
	}
}

const errSuffix = "value should be positive and non-zero"

func (p *Parameters) Validate() error {
	if p.StoreCacheSize <= 0 {
		return fmt.Errorf("invalid store cache size:%s", errSuffix)
	}
	if p.IndexCacheSize <= 0 {
		return fmt.Errorf("invalid indexer cache size:%s", errSuffix)
	}
	if p.WriteBatchSize <= 0 {
		return fmt.Errorf("invalid batch size:%s", errSuffix)
	}
	return nil
}

// WithStoreCacheSize is a functional option that configures the
// `StoreCacheSize` parameter.
func WithStoreCacheSize(size int) Option {
	return func(p *Parameters) {
		p.StoreCacheSize = size
	}
}

// WithIndexCacheSize is a functional option that configures the
// `IndexCacheSize` parameter.
func WithIndexCacheSize(size int) Option {
	return func(p *Parameters) {
		p.IndexCacheSize = size
	}
}

// WithWriteBatchSize is a functional option that configures the
// `WriteBatchSize` parameter.
func WithWriteBatchSize(size int) Option {
	return func(p *Parameters) {
		p.WriteBatchSize = size
	}
}
