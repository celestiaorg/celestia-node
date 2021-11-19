package node

import (
	"context"

	"go.uber.org/fx"

	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/services"
)

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *Config, repo Repository) fx.Option {
	return fx.Options(
		baseComponents(cfg, repo),
		fx.Provide(services.DASer),
		fx.Provide(services.HeaderExchangeP2P(cfg.Services)),
	)
}

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *Config, repo Repository) fx.Option {
	return fx.Options(
		baseComponents(cfg, repo),
		nodecore.Components(cfg.Core, repo.Core),
		fx.Provide(services.BlockService),
		fx.Provide(services.HeaderExchangeP2PServer),
	)
}

// baseComponents keeps all the common components shared between different Node types.
func baseComponents(cfg *Config, repo Repository) fx.Option {
	return fx.Options(
		fx.Provide(context.Background),
		fx.Supply(cfg),
		fx.Supply(repo.Config),
		fx.Provide(repo.Datastore),
		fx.Provide(repo.Keystore),
		fx.Provide(services.ShareService),
		fx.Provide(services.HeaderService),
		fx.Provide(services.HeaderStore),
		fx.Provide(services.HeaderSyncer(cfg.Services)),
		fx.Provide(services.LightAvailability), // TODO(@Wondertan): Move to light once FullAvailability is implemented
		p2p.Components(cfg.P2P),
	)
}
