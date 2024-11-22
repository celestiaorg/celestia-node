package full

import (
	"time"

	"github.com/celestiaorg/celestia-node/share/availability"
)

type params struct {
	storageWindow time.Duration
	archival      bool
}

// Option is a function that configures light availability Parameters
type Option func(*params)

// DefaultParameters returns the default Parameters' configuration values
// for the light availability implementation
func defaultParams() *params {
	return &params{
		storageWindow: availability.StorageWindow,
		archival:      false,
	}
}

func WithArchivalMode() Option {
	return func(p *params) {
		p.archival = true
	}
}

// WithStorageWindow is a functional option to set the storage window
// to something other than the default for the full availability implementation
func WithStorageWindow(window time.Duration) Option {
	return func(p *params) {
		p.storageWindow = window
	}
}
