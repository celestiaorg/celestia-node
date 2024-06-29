package cache

import (
	"context"
	"errors"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"

	eds "github.com/celestiaorg/celestia-node/share/new_eds"
)

var (
	log   = logging.Logger("store/cache")
	meter = otel.Meter("store_cache")
)

var ErrCacheMiss = errors.New("accessor not found in blockstore cache")

type OpenAccessorFn func(context.Context) (eds.AccessorStreamer, error)

type key = uint64

// Cache is an interface that defines the basic Cache operations.
type Cache interface {
	// Get returns the eds.AccessorStreamer for the given key.
	Get(key) (eds.AccessorStreamer, error)

	// GetOrLoad attempts to get an item from the Cache and, if not found, invokes
	// the provided loader function to load it into the Cache.
	GetOrLoad(context.Context, key, OpenAccessorFn) (eds.AccessorStreamer, error)

	// Remove removes an item from Cache.
	Remove(key) error

	// EnableMetrics enables metrics in Cache
	EnableMetrics() error
}
