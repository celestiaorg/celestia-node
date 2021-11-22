package core

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/header"
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
func DefaultConfig() Config {
	return Config{
		Remote: false,
	}
}

// Components collects all the components and services related to managing the relationship with the Core node.
func Components(cfg Config, loader core.RepoLoader) fx.Option {
	return fx.Options(
		fx.Provide(core.NewBlockFetcher),
		fxutil.ProvideAs(header.NewCoreExchange, new(header.Exchange)),
		fxutil.ProvideIf(cfg.Remote, func() (core.Client, error) {
			return RemoteClient(cfg)
		}),
		fxutil.InvokeIf(cfg.Remote, func(c core.Client) error {
			return c.Start()
		}),
		fxutil.ProvideIf(!cfg.Remote, func() (core.Client, error) {
			repo, err := loader()
			if err != nil {
				return nil, err
			}

			cfg, err := repo.Config()
			if err != nil {
				return nil, err
			}

			return core.NewEmbedded(cfg)
		}),
	)
}

// RemoteClient provides a constructor for core.Client over RPC.
func RemoteClient(cfg Config) (core.Client, error) {
	return core.NewRemote(cfg.RemoteConfig.Protocol, cfg.RemoteConfig.RemoteAddr)
}
