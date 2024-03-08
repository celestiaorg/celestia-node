package core

import "github.com/celestiaorg/celestia-node/nodebuilder/p2p"

type Option func(*params)

type params struct {
	chainID string
	metrics bool
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
