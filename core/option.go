package core

import (
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/availability"
)

type Option func(*params)

type params struct {
	metrics bool
	chainID string

	availabilityWindow time.Duration
	pruningEnabled     bool
}

func defaultParams() params {
	return params{
		availabilityWindow: availability.StorageWindow,
		// TODO @renaynay: eventually once pruning is default, set to true
		pruningEnabled: false,
	}
}

// WithMetrics is a functional option that enables metrics
// inside the core package.
func WithMetrics() Option {
	return func(p *params) {
		p.metrics = true
	}
}

func WithChainID(id p2p.Network) Option {
	return func(p *params) {
		p.chainID = id.String()
	}
}

func WithAvailabilityWindow(window time.Duration) Option {
	return func(p *params) {
		p.availabilityWindow = window
	}
}

func WithPruningEnabled() Option {
	return func(p *params) {
		p.pruningEnabled = true
	}
}
