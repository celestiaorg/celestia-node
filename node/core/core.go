package core

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
)

// Config combines all configuration fields for Core subsystem.
type Config struct {
	Remote       bool
	RemoteConfig struct {
		Protocol   string
		RemoteAddr string
	}
	EmbeddedConfig *core.Config
}

// DefaultConfig returns default configuration for Core subsystem.
func DefaultConfig() *Config {
	return &Config{
		EmbeddedConfig: core.DefaultConfig(),
		Remote:         false,
	}
}

// Components collects all the components and services related to the Core node.
func Components(cfg *Config) fx.Option {
	return fx.Options(
		fx.Provide(func() (core.Client, error) {
			if cfg.Remote {
				return core.NewRemote(cfg.RemoteConfig.Protocol, cfg.RemoteConfig.RemoteAddr)
			}

			return core.NewEmbedded(cfg.EmbeddedConfig)
		}),
	)
}
