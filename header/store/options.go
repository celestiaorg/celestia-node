package store

import "errors"

// Option is the functional option that is applied to the store instance
// to configure store parameters.
type Option func(*Parameters)

// Parameters is the set of parameters that must be configured for the store.
type Parameters struct {
	// StoreCacheSize defines the amount of max entries allowed in the Header Store cache.
	StoreCacheSize int

	// IndexCacheSize defines the amount of max entries allowed in the Height to Hash index
	// cache.
	IndexCacheSize int

	// WriteBatchSize defines the size of the batched header write.
	// Headers are written in batches not to thrash the underlying Datastore with writes.
	WriteBatchSize int
}

// DefaultParameters returns default params to configure store.
func DefaultParameters() *Parameters {
	return &Parameters{
		StoreCacheSize: 4096,
		IndexCacheSize: 16384,
		WriteBatchSize: 2048,
	}
}

func (p *Parameters) Validate() error {
	if p.StoreCacheSize <= 0 {
		return errors.New("invalid store cache size")
	}
	if p.IndexCacheSize <= 0 {
		return errors.New("invalid indexer cache size")
	}
	if p.WriteBatchSize <= 0 {
		return errors.New("invalid batch size")
	}
	return nil
}

// WithDefaultStoreCacheSize is a functional option that allows to configure
// `StoreCacheSize` parameter.
func WithDefaultStoreCacheSize(size int) Option {
	return func(p *Parameters) {
		p.StoreCacheSize = size
	}
}

// WithDefaultIndexCacheSize is a functional option that allows to configure
// `IndexCacheSize` parameter.
func WithDefaultIndexCacheSize(size int) Option {
	return func(p *Parameters) {
		p.IndexCacheSize = size
	}
}

// WithDefaultWriteBatchSize is a functional option that allows to configure
// `WriteBatchSize` parameter.
func WithDefaultWriteBatchSize(size int) Option {
	return func(p *Parameters) {
		p.WriteBatchSize = size
	}
}
