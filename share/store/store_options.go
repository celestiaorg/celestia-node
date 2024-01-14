package store

import (
	"fmt"
)

type Parameters struct {
	// RecentBlocksCacheSize is the size of the cache for recent blocks.
	RecentBlocksCacheSize int

	// BlockstoreCacheSize is the size of the cache for blockstore requested accessors.
	BlockstoreCacheSize int
}

// DefaultParameters returns the default configuration values for the EDS store parameters.
func DefaultParameters() *Parameters {
	return &Parameters{
		RecentBlocksCacheSize: 10,
		BlockstoreCacheSize:   128,
	}
}

func (p *Parameters) Validate() error {
	if p.RecentBlocksCacheSize < 1 {
		return fmt.Errorf("eds: recent blocks cache size must be positive")
	}

	if p.BlockstoreCacheSize < 1 {
		return fmt.Errorf("eds: blockstore cache size must be positive")
	}
	return nil
}
