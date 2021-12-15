package node

import (
	"context"

	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/services"
)

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *Config, repo Repository) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(Light),
		baseComponents(cfg, repo),
		fxutil.Provide(services.DASer),
		fxutil.Provide(services.HeaderExchangeP2P(cfg.Services)),
	)
}

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *Config, repo Repository) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(Full),
		baseComponents(cfg, repo),
		nodecore.Components(cfg.Core, repo.Core),
		fxutil.Provide(services.BlockService),
		fxutil.Invoke(services.StartHeaderExchangeP2PServer),
	)
}

// baseComponents keeps all the common components shared between different Node types.
func baseComponents(cfg *Config, repo Repository) fxutil.Option {
	return fxutil.Options(
		fxutil.Provide(context.Background),
		fxutil.Supply(cfg),
		fxutil.Supply(repo.Config),
		fxutil.Provide(repo.Datastore),
		fxutil.Provide(repo.Keystore),
		fxutil.Provide(services.ShareService),
		fxutil.Provide(services.HeaderService),
		fxutil.Provide(services.HeaderStore),
		fxutil.Provide(services.HeaderSyncer(cfg.Services)),
		fxutil.Provide(services.LightAvailability), // TODO(@Wondertan): Move to light once FullAvailability is implemented
		p2p.Components(cfg.P2P),
	)
}
