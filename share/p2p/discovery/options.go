package discovery

import (
	"fmt"
	"time"
)

// Parameters is the set of Parameters that must be configured for the Discovery module
type Parameters struct {
	// PeersLimit defines the soft limit of FNs to connect to via discovery.
	// Set 0 to disable.
	PeersLimit uint
	// AdvertiseInterval is a interval between advertising sessions.
	// Set -1 to disable.
	// NOTE: only full and bridge can advertise themselves.
	AdvertiseInterval time.Duration
}

// Option is a function that configures Discovery Parameters
type Option func(*Parameters)

// DefaultParameters returns the default Parameters' configuration values
// for the Discovery module
func DefaultParameters() Parameters {
	return Parameters{
		PeersLimit: 5,
		// based on https://github.com/libp2p/go-libp2p-kad-dht/pull/793
		AdvertiseInterval: time.Hour * 22,
	}
}

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.AdvertiseInterval <= 0 {
		return fmt.Errorf(
			"discovery: invalid option: value AdvertiseInterval %s, %s",
			"is 0 or negative.",
			"value must be positive",
		)
	}

	return nil
}

// WithPeersLimit is a functional option that Discovery
// uses to set the PeersLimit configuration param
func WithPeersLimit(peersLimit uint) Option {
	return func(p *Parameters) {
		p.PeersLimit = peersLimit
	}
}

// WithAdvertiseInterval is a functional option that Discovery
// uses to set the AdvertiseInterval configuration param
func WithAdvertiseInterval(advInterval time.Duration) Option {
	return func(p *Parameters) {
		p.AdvertiseInterval = advInterval
	}
}
