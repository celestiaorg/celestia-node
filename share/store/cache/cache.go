package cache

import (
	"context"
	"errors"

	logging "github.com/ipfs/go-log/v2"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/celestia-node/share/store/file"
)

var (
	log   = logging.Logger("share/eds/cache")
	meter = otel.Meter("eds_store_cache")
)

var (
	ErrCacheMiss = errors.New("accessor not found in blockstore cache")
)

type OpenFileFn func(context.Context) (file.EdsFile, error)

type key = uint64

// Cache is an interface that defines the basic Cache operations.
type Cache interface {
	// Get returns the EDS file for the given key.
	Get(key) (file.EdsFile, error)

	// GetOrLoad attempts to get an item from the Cache and, if not found, invokes
	// the provided loader function to load it into the Cache.
	GetOrLoad(context.Context, key, OpenFileFn) (file.EdsFile, error)

	// Remove removes an item from Cache.
	Remove(key) error

	// EnableMetrics enables metrics in Cache
	EnableMetrics() error
}
