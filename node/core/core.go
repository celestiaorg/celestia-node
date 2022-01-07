package core

import (
	"context"

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
func Components(cfg Config, loader core.RepoLoader) fxutil.Option {
	return fxutil.Options(
		fxutil.Provide(core.NewBlockFetcher),
		fxutil.ProvideAs(header.NewCoreExchange, new(header.Exchange)),
		fxutil.ProvideIf(cfg.Remote, func(lc fx.Lifecycle) (core.Client, error) {
			client, err := RemoteClient(cfg)
			if err != nil {
				return nil, err
			}

			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					return client.Start()
				},
				OnStop: func(_ context.Context) error {
					return client.Stop()
				},
			})

			return client, nil
		}),
		fxutil.ProvideIf(!cfg.Remote, func(lc fx.Lifecycle) (core.Client, error) {
			store, err := loader()
			if err != nil {
				return nil, err
			}

			cfg, err := store.Config()
			if err != nil {
				return nil, err
			}

			client, err := core.NewEmbedded(cfg)
			if err != nil {
				return nil, err
			}

			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					return client.Start()
				},
				OnStop: func(_ context.Context) error {
					return client.Stop()
				},
			})

			return client, nil
		}),
	)
}

// RemoteClient provides a constructor for core.Client over RPC.
func RemoteClient(cfg Config) (core.Client, error) {
	return core.NewRemote(cfg.RemoteConfig.Protocol, cfg.RemoteConfig.RemoteAddr)
}
