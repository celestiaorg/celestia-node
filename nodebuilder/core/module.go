package core

import (
	"context"

	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	headp2p "github.com/celestiaorg/go-header/p2p"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	modshare "github.com/celestiaorg/celestia-node/nodebuilder/share"
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
	case node.Light:
		return fx.Module("core", baseComponents)
	case node.Bridge:
		return fx.Module("core",
			baseComponents,
			fx.Provide(core.NewBlockFetcher),
			fx.Provide(func(
				fetcher *core.BlockFetcher,
				store *store.Store,
				construct header.ConstructFn,
				p2pEx *headp2p.Exchange[*header.ExtendedHeader],
				chainID p2p.Network,
				opts []core.Option,
			) (*core.Exchange, error) {
				opts = append(opts, core.WithChainID(chainID))

				if MetricsEnabled {
					opts = append(opts, core.WithMetrics())
				}
				// Add P2P exchange fallback for when core doesn't have blocks
				// Only headers will be fetched; EDS downloading is handled by DASer
				opts = append(opts, core.WithP2PExchange(p2pEx))
				return core.NewExchange(fetcher, store, construct, opts...)
			}),
			fxutil.ProvideAs(func(
				coreEx *core.Exchange,
				p2pEx *headp2p.Exchange[*header.ExtendedHeader],
				window modshare.Window,
			) (*core.RoutingExchange, error) {
				return core.NewRoutingExchange(
					coreEx,
					p2pEx,
					window.Duration(),
					p2p.BlockTime,
				)
			},
				new(libhead.Exchange[*header.ExtendedHeader])),
			fx.Invoke(fx.Annotate(
				func(
					bcast libhead.Broadcaster[*header.ExtendedHeader],
					fetcher *core.BlockFetcher,
					pubsub *shrexsub.PubSub,
					construct header.ConstructFn,
					store *store.Store,
					chainID p2p.Network,
					opts []core.Option,
				) (*core.Listener, error) {
					opts = append(opts, core.WithChainID(chainID))

					if MetricsEnabled {
						opts = append(opts, core.WithMetrics())
					}

					return core.NewListener(bcast, fetcher, pubsub.Broadcast, construct, store, p2p.BlockTime, opts...)
				},
				fx.OnStart(func(ctx context.Context, listener *core.Listener) error {
					return listener.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, listener *core.Listener) error {
					return listener.Stop(ctx)
				}),
			)),
		)
	default:
		panic("invalid node type")
	}
}
