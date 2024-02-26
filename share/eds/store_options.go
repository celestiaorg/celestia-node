package eds

import (
	"errors"
	"time"
)

type Parameters struct {
	// GC performs DAG store garbage collection by reclaiming transient files of
	// shards that are currently available but inactive, or errored.
	// We don't use transient files right now, so GC is turned off by default.
	GCInterval time.Duration

	// RecentBlocksCacheSize is the size of the cache for recent blocks.
	RecentBlocksCacheSize int

	// BlockstoreCacheSize is the size of the cache for blockstore requested accessors.
	BlockstoreCacheSize int
}

// DefaultParameters returns the default configuration values for the EDS store parameters.
func DefaultParameters() *Parameters {
	return &Parameters{
		GCInterval:            0,
		RecentBlocksCacheSize: 10,
		BlockstoreCacheSize:   128,
	}
}

func (p *Parameters) Validate() error {
	if p.GCInterval < 0 {
		return errors.New("eds: GC interval cannot be negative")
	}

	if p.RecentBlocksCacheSize < 1 {
		return errors.New("eds: recent blocks cache size must be positive")
	}

	if p.BlockstoreCacheSize < 1 {
		return errors.New("eds: blockstore cache size must be positive")
	}
	return nil
}
