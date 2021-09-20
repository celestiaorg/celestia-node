package core

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
)

// Config combines all configuration fields for managing the relationship with a Core node.
type Config struct {
	Remote       bool
	RemoteConfig struct {
		Protocol   string
		RemoteAddr string
	}
}

// DefaultConfig returns default configuration for Core subsystem.
func DefaultConfig() *Config {
	return &Config{
		Remote: false,
	}
}

// Components collects all the components and services related to managing the relationship with the Core node.
func Components(cfg *Config) fx.Option {
	return fx.Options(
		fxutil.ProvideIf(cfg.Remote, RemoteClient),
		fxutil.ProvideIf(!cfg.Remote, core.NewEmbedded),
	)
}

// RemoteClient provides a constructor for core.Client over RPC.
func RemoteClient(cfg *Config) (core.Client, error) {
	return core.NewRemote(cfg.RemoteConfig.Protocol, cfg.RemoteConfig.RemoteAddr)
}
