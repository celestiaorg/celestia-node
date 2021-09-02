package p2p

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
)

// Components collects all the components and services related to p2p.
func Components(cfg *config.Config) fx.Option {
	return fx.Options(
		fx.Provide(Host()),
		fx.Provide(AddrsFactory(cfg.P2P.AnnounceAddresses, cfg.P2P.NoAnnounceAddresses)),
		fx.Invoke(Listen(cfg.P2P.ListenAddresses)),
	)
}
