package p2p

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/metrics"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("module/p2p")

// ConstructModule collects all the components and services related to p2p.
func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
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
		fx.Provide(metrics.NewBandwidthCounter),
		fx.Provide(newModule),
		fx.Invoke(Listen(cfg.ListenAddresses)),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"p2p",
			baseComponents,
		)
	default:
		panic("invalid node type")
	}
}
