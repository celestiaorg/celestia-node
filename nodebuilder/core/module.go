package core

import (
	"context"

	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

// ConstructModule collects all the components and services related to managing the relationship
// with the Core node.
func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Options(options...),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module("core", baseComponents)
	case node.Bridge:
		return fx.Module("core",
			baseComponents,
			fx.Provide(core.NewBlockFetcher),
			fxutil.ProvideAs(
				func(
					fetcher *core.BlockFetcher,
					store *eds.Store,
					construct header.ConstructFn,
				) (*core.Exchange, error) {
					var opts []core.Option
					if MetricsEnabled {
						opts = append(opts, core.WithMetrics())
					}

					return core.NewExchange(fetcher, store, construct, opts...)
				},
				new(libhead.Exchange[*header.ExtendedHeader])),
			fx.Invoke(fx.Annotate(
				func(
					bcast libhead.Broadcaster[*header.ExtendedHeader],
					fetcher *core.BlockFetcher,
					pubsub *shrexsub.PubSub,
					construct header.ConstructFn,
					store *eds.Store,
					chainID p2p.Network,
				) (*core.Listener, error) {
					var opts []core.Option
					if MetricsEnabled {
						opts = append(opts, core.WithMetrics(), core.WithChainID(chainID))
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
			fx.Provide(fx.Annotate(
				remote,
				fx.OnStart(func(ctx context.Context, client core.Client) error {
					return client.Start()
				}),
				fx.OnStop(func(ctx context.Context, client core.Client) error {
					return client.Stop()
				}),
			)),
		)
	default:
		panic("invalid node type")
	}
}
