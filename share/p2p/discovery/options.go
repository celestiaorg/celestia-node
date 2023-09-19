package discovery

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	// fullNodesTag is the namespace where full nodes advertise and discover each other.
	fullNodesTag = "full"
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
	// onUpdatedPeers will be called on peer set changes
	onUpdatedPeers OnUpdatedPeers
	// Tag is used as rondezvous point for discovery service
	Tag string
}

// Option is a function that configures Discovery Parameters
type Option func(*Parameters)

// DefaultParameters returns the default Parameters' configuration values
// for the Discovery module
func DefaultParameters() *Parameters {
	return &Parameters{
		PeersLimit: 5,
		// based on https://github.com/libp2p/go-libp2p-kad-dht/pull/793
		AdvertiseInterval: time.Hour * 22,
		//TODO: remove fullNodesTag default value once multiple tags are supported
		Tag:            fullNodesTag,
		onUpdatedPeers: func(peer.ID, bool) {},
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

	if p.PeersLimit <= 0 {
		return fmt.Errorf(
			"discovery: invalid option: value PeersLimit %s, %s",
			"is negative.",
			"value must be positive",
		)
	}

	if p.Tag == "" {
		return fmt.Errorf(
			"discovery: invalid option: value Tag %s, %s",
			"is empty.",
			"value must be non-empty",
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

// WithOnPeersUpdate chains OnPeersUpdate callbacks on every update of discovered peers list.
func WithOnPeersUpdate(f OnUpdatedPeers) Option {
	return func(p *Parameters) {
		p.onUpdatedPeers = p.onUpdatedPeers.add(f)
	}
}

// WithTag is a functional option that sets the Tag for the discovery service
func WithTag(tag string) Option {
	return func(p *Parameters) {
		p.Tag = tag
	}
}
