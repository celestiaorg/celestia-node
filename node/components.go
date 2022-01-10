package node

import (
	"context"

	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/services"
)

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(Light),
		baseComponents(cfg, store),
		fxutil.Provide(services.DASer),
		fxutil.Provide(services.HeaderExchangeP2P(cfg.Services)),
	)
}

// fullComponents keeps all the components as DI options required to build a Full Node.
func bridgeComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Supply(Bridge),
		baseComponents(cfg, store),
		nodecore.Components(cfg.Core, store.Core),
	)
}

// baseComponents keeps all the common components shared between different Node types.
func baseComponents(cfg *Config, store Store) fxutil.Option {
	return fxutil.Options(
		fxutil.Provide(context.Background),
		fxutil.Supply(cfg),
		fxutil.Supply(store.Config),
		fxutil.Provide(store.Datastore),
		fxutil.Provide(store.Keystore),
		fxutil.Provide(services.ShareService),
		fxutil.Provide(services.HeaderService),
		fxutil.Provide(services.HeaderStore),
		fxutil.Provide(services.HeaderSyncer(cfg.Services)),
		fxutil.Provide(services.P2PSubscriber),
		fxutil.Provide(services.HeaderP2PExchangeServer),
		fxutil.Provide(services.LightAvailability), // TODO(@Wondertan): Move to light once FullAvailability is implemented
		p2p.Components(cfg.P2P),
	)
}
