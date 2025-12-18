package core

import (
	"context"

	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"

	corelib "github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	modpruner "github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

// ConstructModule collects all the components and services related to managing the relationship
// with the Core node.
func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Supply(cfg.EndpointConfig),
		fx.Error(cfgErr),
		fx.Provide(grpcClient),
		fx.Provide(additionalCoreEndpointGrpcClients),
		fx.Options(options...),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module("core", baseComponents)
	case node.Bridge, node.Pin:
		return fx.Module("core",
			baseComponents,
			fx.Provide(corelib.NewBlockFetcher),
			fxutil.ProvideAs(
				func(
					fetcher *corelib.BlockFetcher,
					store *store.Store,
					construct header.ConstructFn,
					prunerCfg *modpruner.Config,
					opts []corelib.Option,
				) (*corelib.Exchange, error) {
					opts = append(opts,
						corelib.WithAvailabilityWindow(prunerCfg.StorageWindow),
					)

					if MetricsEnabled {
						opts = append(opts, corelib.WithMetrics())
					}

					return corelib.NewExchange(fetcher, store, construct, opts...)
				},
				new(libhead.Exchange[*header.ExtendedHeader])),
			fx.Invoke(fx.Annotate(
				func(
					bcast libhead.Broadcaster[*header.ExtendedHeader],
					fetcher *corelib.BlockFetcher,
					pubsub *shrexsub.PubSub,
					construct header.ConstructFn,
					store *store.Store,
					chainID p2p.Network,
					prunerCfg *modpruner.Config,
					opts []corelib.Option,
				) (*corelib.Listener, error) {
					opts = append(opts,
						corelib.WithChainID(chainID),
						corelib.WithAvailabilityWindow(prunerCfg.StorageWindow),
					)

					if MetricsEnabled {
						opts = append(opts, corelib.WithMetrics())
					}

					return corelib.NewListener(bcast, fetcher, pubsub.Broadcast, construct, store, p2p.BlockTime, opts...)
				},
				fx.OnStart(func(ctx context.Context, listener *corelib.Listener) error {
					return listener.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, listener *corelib.Listener) error {
					return listener.Stop(ctx)
				}),
			)),
		)
	default:
		panic("invalid node type")
	}
}
