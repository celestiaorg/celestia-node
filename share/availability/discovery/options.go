package discovery

import (
	"fmt"
	"time"
)

// Parameters is the set of Parameters that must be configured for the Discovery module
type Parameters struct {
	// PeersLimit is max amount of peers that will be Discovered during a Discovery session.
	PeersLimit int

	// DiscInterval is an interval between Discovery sessions.
	DiscoveryInterval time.Duration

	// AdvertiseInterval is an interval between Advertising sessions.
	AdvertiseInterval time.Duration
}

// Option is a function that configures Discovery Parameters
type Option func(*Parameters)

// DefaultParameters returns the default Parameters' configuration values
// for the Discovery module
func DefaultParameters() Parameters {
	return Parameters{
		PeersLimit:        3,
		DiscoveryInterval: time.Second * 30,
		AdvertiseInterval: time.Second * 30,
	}
}

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.PeersLimit < 0 {
		return fmt.Errorf(
			"discovery: invalid option: value PeersLimit %s, %s",
			"is negative.",
			"value must be 0 or positive",
		)
	}

	if p.DiscoveryInterval <= 0 {
		return fmt.Errorf(
			"discovery: invalid option: value DicoveryInterval %s, %s",
			"is 0 or negative.",
			"value must be positive",
		)
	}

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
func WithPeersLimit(peersLimit int) Option {
	return func(p *Parameters) {
		p.PeersLimit = peersLimit
	}
}

// WithDiscoveryInterval is a functional option that Discovery
// uses to set the DiscoveryInterval configuration param
func WithDiscoveryInterval(discInterval time.Duration) Option {
	return func(p *Parameters) {
		p.DiscoveryInterval = discInterval
	}
}

// WithAdvertiseInterval is a functional option that Discovery
// uses to set the AdvertiseInterval configuration param
func WithAdvertiseInterval(advInterval time.Duration) Option {
	return func(p *Parameters) {
		p.AdvertiseInterval = advInterval
	}
}
