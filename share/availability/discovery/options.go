package discovery

import (
	"fmt"
	"time"
)

// Parameters is the set of Parameters that must be configured for the Discovery module
type Parameters struct {
	// PeersLimit is max amount of peers that will be Discovered during a Discovery session.
	PeersLimit uint

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
	if p.PeersLimit <= 0 {
		return fmt.Errorf(
			"discovery: invalid option: value %s was %s, where it should be %s",
			"PeersLimit",
			"<= 0", // current value
			">= 0", // what the value should be
		)
	}

	if p.DiscoveryInterval <= 0 {
		return fmt.Errorf(
			"discovery: invalid option: value %s was %s, where it should be %s",
			"DicoveryInterval",
			"<= 0", // current value
			">= 0", // what the value should be
		)
	}

	if p.AdvertiseInterval <= 0 {
		return fmt.Errorf(
			"discovery: invalid option: value %s was %s, where it should be %s",
			"AdvertiseInterval",
			"<= 0", // current value
			">= 0", // what the value should be
		)
	}

	return nil
}

// WithPeersLimit is a functional option that the Discoverable interface
// implementers use to set the PeersLimit configuration param
//
// To be used with the construction
// example:
//
// NewDiscovery(
//
//	bServ,
//	disc,
//	WithPeersLimit(3),
//
// )
func WithPeersLimit(peersLimit uint) Option {
	return func(p *Parameters) {
		p.PeersLimit = peersLimit
	}
}

// WithDiscoveryInterval is a functional option that the Discoverable interface
// implementers use to set the DiscoveryInterval configuration param
//
// To be used with the construction, see example in WithPeersLimit documentation
func WithDiscoveryInterval(discInterval time.Duration) Option {
	return func(p *Parameters) {
		p.DiscoveryInterval = discInterval
	}
}

// WithAdvertiseInterval is a functional option that the Discoverable interface
// implementers use to set the AdvertiseInterval configuration param
//
// To be used with the construction, see example in WithPeersLimit documentation
func WithAdvertiseInterval(advInterval time.Duration) Option {
	return func(p *Parameters) {
		p.AdvertiseInterval = advInterval
	}
}
