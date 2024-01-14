package cache

import (
	"context"
	"errors"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/store/file"
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

// Key is a unique identifier for an item in the Cache. Either Datahash or Height should be set.
type Key struct {
	Datahash share.DataHash
	Height   uint64
}

// A Key is considered complete if it has both Datahash and Height set.
func (k Key) isComplete() bool {
	return k.Datahash != nil && k.Height != 0
}

type OpenFileFn func(context.Context) (Key, file.EdsFile, error)

// Cache is an interface that defines the basic Cache operations.
type Cache interface {
	// Get returns the EDS file for the given key.
	Get(Key) (file.EdsFile, error)

	// GetOrLoad attempts to get an item from the Cache and, if not found, invokes
	// the provided loader function to load it into the Cache.
	GetOrLoad(context.Context, Key, OpenFileFn) (file.EdsFile, error)

	// Remove removes an item from Cache.
	Remove(Key) error

	// EnableMetrics enables metrics in Cache
	EnableMetrics() error
}
