package header

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/node/node"
)

// WithHeaderConstructFn sets custom func that creates extended header
func WithHeaderConstructFn(construct header.ConstructFn) node.Option {
	return func(sets *node.Settings) {
		sets.Opts = append(sets.Opts, fx.Replace(construct))
	}
}

// WithTrustedHash sets a TrustedHash in the Config, which is used for header sync initialization.
func WithTrustedHash(hash string) node.Option {
	return func(sets *node.Settings) {
		sets.Cfg.Services.TrustedHash = hash
	}
}

// WithTrustedPeers appends new "trusted peers" to fetch headers from to the Config.
func WithTrustedPeers(addr ...string) node.Option {
	return func(sets *node.Settings) {
		sets.Cfg.Services.TrustedPeers = append(sets.Cfg.Services.TrustedPeers, addr...)
	}
}
