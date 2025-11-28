package core

import (
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/availability"
)

type Option func(*params)

type params struct {
	metrics            bool
	chainID            string
	availabilityWindow time.Duration
	archival           bool
	odsOnly            bool
}

func defaultParams() params {
	return params{
		availabilityWindow: availability.StorageWindow,
		archival:           false,
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

func WithArchivalMode() Option {
	return func(p *params) {
		p.archival = true
	}
}

// WithODSOnly is a functional option that enables ODS-only storage mode.
// When enabled, all blocks are stored using PutODS instead of PutODSQ4.
func WithODSOnly() Option {
	return func(p *params) {
		p.odsOnly = true
	}
}
