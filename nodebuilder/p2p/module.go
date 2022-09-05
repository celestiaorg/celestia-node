package p2p

import (
	"go.uber.org/fx"
)

// Module collects all the components and services related to p2p.
func Module(cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.ValidateBasic()

	return fx.Module(
		"p2p",
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(Key),
		fx.Provide(ID),
		fx.Provide(PeerStore),
		fx.Provide(ConnectionManager),
		fx.Provide(ConnectionGater),
		fx.Provide(Host),
		fx.Provide(RoutedHost),
		fx.Provide(PubSub),
		fx.Provide(DataExchange),
		fx.Provide(BlockService),
		fx.Provide(PeerRouting),
		fx.Provide(ContentRouting),
		fx.Provide(AddrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)),
		fx.Invoke(Listen(cfg.ListenAddresses)),
	)
}
