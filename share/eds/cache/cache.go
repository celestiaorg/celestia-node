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

var (
	errCacheMiss = errors.New("accessor not found in blockstore cache")
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
		loader func(context.Context, shard.Key) (Accessor, error),
	) (Accessor, error)

	// Remove removes an item from Cache.
	Remove(shard.Key) error

	// EnableMetrics enables metrics in Cache
	EnableMetrics() error
}

// Accessor is a interface type returned by cache, that allows to read raw data by reader or create
// readblockstore
type Accessor interface {
	Blockstore() (dagstore.ReadBlockstore, error)
	Reader() io.Reader
	io.Closer
}
