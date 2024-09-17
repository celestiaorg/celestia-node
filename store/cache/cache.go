package cache

import (
	"context"
	"errors"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/celestia-node/share/eds"
)

var (
	log   = logging.Logger("store/cache")
	meter = otel.Meter("store_cache")
)

var ErrCacheMiss = errors.New("accessor not found in cache")

type OpenAccessorFn func(context.Context) (eds.AccessorStreamer, error)

// Cache is an interface that defines the basic Cache operations.
type Cache interface {
	// Get returns the eds.AccessorStreamer for the given height.
	Get(height uint64) (eds.AccessorStreamer, error)

	// GetOrLoad attempts to get an item from the Cache and, if not found, invokes
	// the provided loader function to load it into the Cache.
	GetOrLoad(ctx context.Context, height uint64, open OpenAccessorFn) (eds.AccessorStreamer, error)

	// Remove removes an item from Cache.
	Remove(height uint64) error

	// EnableMetrics enables metrics in Cache
	EnableMetrics() (unreg func() error, err error)
}
