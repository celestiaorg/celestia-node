package p2p

import (
	logging "github.com/ipfs/go-log/v2"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

var log = logging.Logger("module/p2p")

// ConstructModule collects all the components and services related to p2p.
func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// validate config values before constructing module
	_, err := parseListenAddrs(cfg.ListenAddresses)
	if err != nil {
		return fx.Error(err)
	}

	baseComponents := fx.Options(
		fx.Supply(cfg),
		fx.Provide(Key),
		fx.Provide(id),
		fx.Provide(peerStore),
		fx.Provide(connectionManager),
		fx.Provide(connectionGater),
		fx.Provide(newHost),
		fx.Provide(routedHost),
		fx.Provide(pubSub),
		fx.Provide(ipld.NewBlockservice),
		fx.Provide(peerRouting),
		fx.Provide(newDHT),
		fx.Provide(addrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)),
		fx.Provide(metrics.NewBandwidthCounter),
		fx.Provide(newModule),
		// Construct the routed host eagerly so the P2P lifecycle hooks are registered.
		fx.Invoke(func(_ hst.Host) {}),
		fx.Provide(resourceManager),
		fx.Provide(resourceManagerOpt(allowList)),
	)

	switch tp {
	case node.Bridge:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(bridgeResources),
			fx.Invoke(reachabilityCheck),
			fx.Invoke(connectToBootstrappers),
		)
	case node.Light:
		return fx.Module(
			"p2p",
			baseComponents,
			fx.Provide(autoscaleResources),
		)
	default:
		panic("invalid node type")
	}
}
