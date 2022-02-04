package node

import (
	"context"
	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/node/services"
	statecomponents "github.com/celestiaorg/celestia-node/node/state"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/state"
	lens "github.com/strangelove-ventures/lens/client"
)

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *Config, store Store) fxutil.Option {
	// condition for adding state-related components
	trustedPeerExists := cfg.Services.TrustedPeer != ""
	opts := fxutil.Options(
		fxutil.Supply(Light),
		baseComponents(cfg, store),
		fxutil.Provide(services.DASer),
		fxutil.Provide(services.HeaderExchangeP2P(cfg.Services)),
		fxutil.ProvideIf(trustedPeerExists, func() (*lens.ChainClient, error) {
			return statecomponents.ChainClient(cfg.Core, store.Path())
		}),
		fxutil.ProvideIf(trustedPeerExists, state.NewCoreAccessor),
	)
	return opts
}

// bridgeComponents keeps all the components as DI options required to build a Bridge Node.
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
		// TODO @renaynay: once full node type is defined, add FullAvailability
		//  to full node and LightAvailability to light node. Bridge node does
		//  not need Availability.
		fxutil.Provide(services.LightAvailability),
		fxutil.Provide(services.HeaderService),
		fxutil.Provide(services.HeaderStore),
		fxutil.Provide(services.HeaderSyncer(cfg.Services)),
		fxutil.ProvideAs(services.P2PSubscriber, new(header.Broadcaster), new(header.Subscriber)),
		fxutil.Provide(services.HeaderP2PExchangeServer),
		p2p.Components(cfg.P2P),
	)
}
