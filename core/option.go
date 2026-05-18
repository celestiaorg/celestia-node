package core

import (
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/availability"
)

type Option func(*params)

type params struct {
	metrics            bool
	chainID            string
	availabilityWindow time.Duration
	archival           bool
	p2pExchange        libhead.Exchange[*header.ExtendedHeader]
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

// WithP2PExchange sets the P2P exchange for fallback when core doesn't have blocks.
// When core is unavailable, only headers will be fetched via P2P.
// The EDS data will be downloaded later by the DASer component.
func WithP2PExchange(ex libhead.Exchange[*header.ExtendedHeader]) Option {
	return func(p *params) {
		p.p2pExchange = ex
	}
}
