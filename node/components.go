package node

import (
	"context"

	"go.uber.org/fx"

	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/services"
)

// lightComponents keeps all the components as DI options required to build a Light Node.
func lightComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		baseComponents(cfg, store),
		fx.Provide(services.DASer),
		fx.Provide(services.HeaderExchangeP2P(cfg.Services)),
	)
}

// bridgeComponents keeps all the components as DI options required to build a Bridge Node.
func bridgeComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		baseComponents(cfg, store),
		nodecore.Components(cfg.Core, store.Core),
		fx.Provide(services.BlockService),
		fx.Invoke(services.StartHeaderExchangeP2PServer),
	)
}

// baseComponents keeps all the common components shared between different Node types.
func baseComponents(cfg *Config, store Store) fx.Option {
	return fx.Options(
		fx.Provide(context.Background),
		fx.Supply(cfg),
		fx.Supply(store.Config),
		fx.Provide(store.Datastore),
		fx.Provide(store.Keystore),
		fx.Provide(services.ShareService),
		fx.Provide(services.HeaderService),
		fx.Provide(services.HeaderStore),
		fx.Provide(services.HeaderSyncer(cfg.Services)),
		fx.Provide(services.LightAvailability), // TODO(@Wondertan): Move to light once FullAvailability is implemented
		p2p.Components(cfg.P2P),
	)
}
