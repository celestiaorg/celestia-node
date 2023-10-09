package discovery

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
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

// options is the set of options that can be configured for the Discovery module
type options struct {
	// onUpdatedPeers will be called on peer set changes
	onUpdatedPeers OnUpdatedPeers
}

// Option is a function that configures Discovery Parameters
type Option func(*options)

// DefaultParameters returns the default Parameters' configuration values
// for the Discovery module
func DefaultParameters() *Parameters {
	return &Parameters{
		PeersLimit:        5,
		AdvertiseInterval: time.Hour,
	}
}

// Validate validates the values in Parameters
func (p *Parameters) Validate() error {
	if p.PeersLimit <= 0 {
		return fmt.Errorf("discovery: peers limit cannot be zero or negative")
	}

	if p.AdvertiseInterval <= 0 {
		return fmt.Errorf("discovery: advertise interval cannot be zero or negative")
	}
	return nil
}

// WithOnPeersUpdate chains OnPeersUpdate callbacks on every update of discovered peers list.
func WithOnPeersUpdate(f OnUpdatedPeers) Option {
	return func(p *options) {
		p.onUpdatedPeers = p.onUpdatedPeers.add(f)
	}
}

func newOptions(opts ...Option) *options {
	defaults := &options{
		onUpdatedPeers: func(peer.ID, bool) {},
	}

	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}
