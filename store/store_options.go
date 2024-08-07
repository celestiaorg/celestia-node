package store

import (
	"errors"
)

type Parameters struct {
	// RecentBlocksCacheSize is the size of the combinedCache for recent blocks.
	RecentBlocksCacheSize int
}

// DefaultParameters returns the default configuration values for the EDS store parameters.
func DefaultParameters() *Parameters {
	return &Parameters{
		RecentBlocksCacheSize: 10,
	}
}

func (p *Parameters) Validate() error {
	if p.RecentBlocksCacheSize < 0 {
		return errors.New("recent eds combinedCache size cannot be negative")
	}
	return nil
}
