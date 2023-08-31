package cache

import (
	"context"
	"errors"
	"io"

	"github.com/filecoin-project/dagstore"
	"github.com/filecoin-project/dagstore/shard"
	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"
)

var (
	log   = logging.Logger("share/eds/cache")
	meter = otel.Meter("eds_store_cache")
)

const (
	cacheFoundKey = "found"
	failedKey     = "failed"
)

var (
	ErrCacheMiss = errors.New("accessor not found in blockstore cache")
)

// Cache is an interface that defines the basic Cache operations.
type Cache interface {
	// Get retrieves an item from the Cache.
	Get(shard.Key) (Accessor, error)

	// GetOrLoad attempts to get an item from the Cache and, if not found, invokes
	// the provided loader function to load it into the Cache.
	GetOrLoad(
		ctx context.Context,
		key shard.Key,
		loader func(context.Context, shard.Key) (AccessorProvider, error),
	) (Accessor, error)

	// Remove removes an item from Cache.
	Remove(shard.Key) error

	// EnableMetrics enables metrics in Cache
	EnableMetrics() error
}

type Accessor interface {
	ReadCloser() io.ReadCloser
	Blockstore() (*BlockstoreCloser, error)
}

// AccessorProvider defines interface used from dagstore.ShardAccessor to allow easy testing
type AccessorProvider interface {
	Reader() io.Reader
	Blockstore() (dagstore.ReadBlockstore, error)
	io.Closer
}

type BlockstoreCloser struct {
	dagstore.ReadBlockstore
	io.Closer
}

var _ Cache = (*NoopCache)(nil)

// NoopCache implements noop version of Cache interface
type NoopCache struct{}

func (n NoopCache) Get(shard.Key) (Accessor, error) {
	return nil, ErrCacheMiss
}

func (n NoopCache) GetOrLoad(
	context.Context, shard.Key,
	func(context.Context, shard.Key) (AccessorProvider, error),
) (Accessor, error) {
	return nil, nil
}

func (n NoopCache) Remove(shard.Key) error {
	return nil
}

func (n NoopCache) EnableMetrics() error {
	return nil
}
