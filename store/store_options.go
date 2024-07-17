package store

import (
	"errors"
)

type Parameters struct {
	// RecentBlocksCacheSize is the size of the cache for recent blocks.
	RecentBlocksCacheSize int

	// AvailabilityCacheSize is the size of the cache for accessors requested for serving availability
	// samples.
	AvailabilityCacheSize int
}

// DefaultParameters returns the default configuration values for the EDS store parameters.
func DefaultParameters() *Parameters {
	return &Parameters{
		RecentBlocksCacheSize: 10,
		AvailabilityCacheSize: 128,
	}
}

func (p *Parameters) Validate() error {
	if p.RecentBlocksCacheSize < 1 {
		return errors.New("recent blocks cache size must be positive")
	}

	if p.AvailabilityCacheSize < 1 {
		return errors.New("blockstore cache size must be positive")
	}
	return nil
}
