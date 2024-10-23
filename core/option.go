package core

import (
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/pruner/archival"
)

type Option func(*params)

type params struct {
	metrics            bool
	chainID            string
	availabilityWindow time.Duration
}

func defaultParams() params {
	return params{
		availabilityWindow: archival.Window,
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
