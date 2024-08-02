package share

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	lightprune "github.com/celestiaorg/celestia-node/pruner/light"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexeds"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexnd"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate(tp)

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Options(options...),
		fx.Provide(newShareModule),
		fx.Provide(bitswap.NewGetter),
		availabilityComponents(tp, cfg),
		shrexComponents(tp, cfg),
		peerComponents(tp, cfg),
	)

	switch tp {
	case node.Bridge:
		return fx.Module(
			"share",
			baseComponents,
			edsStoreComponents(cfg),
			fx.Provide(bridgeGetter),
		)
	case node.Full:
		return fx.Module(
			"share",
			baseComponents,
			edsStoreComponents(cfg),
			fx.Provide(fullGetter),
		)
	case node.Light:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(lightGetter),
		)
	default:
		panic("invalid node type")
	}
}

func shrexComponents(tp node.Type, cfg *Config) fx.Option {
	opts := fx.Options(
		fx.Provide(
			func(ctx context.Context, h host.Host, network modp2p.Network) (*shrexsub.PubSub, error) {
				return shrexsub.NewPubSub(ctx, h, network.String())
			}),
		// shrex-nd client
		fx.Provide(
			func(host host.Host, network modp2p.Network) (*shrexnd.Client, error) {
				cfg.ShrExNDParams.WithNetworkID(network.String())
				return shrexnd.NewClient(cfg.ShrExNDParams, host)
			},
		),

		// shrex-eds client
		fx.Provide(
			func(host host.Host, network modp2p.Network) (*shrexeds.Client, error) {
				cfg.ShrExEDSParams.WithNetworkID(network.String())
				return shrexeds.NewClient(cfg.ShrExEDSParams, host)
			},
		),

		// shrex-getter
		fx.Provide(fx.Annotate(
			func(
				edsClient *shrexeds.Client,
				ndClient *shrexnd.Client,
				managers map[string]*peers.Manager,
			) *shrex_getter.Getter {
				return shrex_getter.NewGetter(
					edsClient,
					ndClient,
					managers[fullNodesTag],
					managers[archivalNodesTag],
					lightprune.Window,
				)
			},
			fx.OnStart(func(ctx context.Context, getter *shrex_getter.Getter) error {
				return getter.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, getter *shrex_getter.Getter) error {
				return getter.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light:
		return fx.Options(
			opts,
			// shrexsub broadcaster stub for daser
			fx.Provide(func() shrexsub.BroadcastFn {
				return func(context.Context, shrexsub.Notification) error {
					return nil
				}
			}),
		)
	case node.Full:
		return fx.Options(
			opts,
			shrexServerComponents(cfg),
			fx.Provide(store.NewGetter),
			fx.Provide(func(shrexSub *shrexsub.PubSub) shrexsub.BroadcastFn {
				return shrexSub.Broadcast
			}),
		)
	case node.Bridge:
		return fx.Options(
			opts,
			shrexServerComponents(cfg),
			fx.Provide(store.NewGetter),
			fx.Provide(func(shrexSub *shrexsub.PubSub) shrexsub.BroadcastFn {
				return shrexSub.Broadcast
			}),
			fx.Invoke(func(lc fx.Lifecycle, sub *shrexsub.PubSub) error {
				lc.Append(fx.Hook{
					OnStart: sub.Start,
					OnStop:  sub.Stop,
				})
				return nil
			}),
		)
	default:
		panic("invalid node type")
	}
}

func shrexServerComponents(cfg *Config) fx.Option {
	return fx.Options(
		fx.Invoke(func(_ *shrexeds.Server, _ *shrexnd.Server) {}),
		fx.Provide(fx.Annotate(
			func(host host.Host, store *store.Store, network modp2p.Network) (*shrexeds.Server, error) {
				cfg.ShrExEDSParams.WithNetworkID(network.String())
				return shrexeds.NewServer(cfg.ShrExEDSParams, host, store)
			},
			fx.OnStart(func(ctx context.Context, server *shrexeds.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *shrexeds.Server) error {
				return server.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(
			func(
				host host.Host,
				store *store.Store,
				network modp2p.Network,
			) (*shrexnd.Server, error) {
				cfg.ShrExNDParams.WithNetworkID(network.String())
				return shrexnd.NewServer(cfg.ShrExNDParams, host, store)
			},
			fx.OnStart(func(ctx context.Context, server *shrexnd.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *shrexnd.Server) error {
				return server.Stop(ctx)
			})),
		),
	)
}

func edsStoreComponents(cfg *Config) fx.Option {
	return fx.Options(
		fx.Provide(fx.Annotate(
			func(path node.StorePath) (*store.Store, error) {
				return store.NewStore(cfg.EDSStoreParams, string(path))
			},
			fx.OnStop(func(ctx context.Context, store *store.Store) error {
				return store.Stop(ctx)
			}),
		)),
	)
}

func availabilityComponents(tp node.Type, cfg *Config) fx.Option {
	switch tp {
	case node.Light:
		return fx.Options(
			fx.Provide(fx.Annotate(
				func(getter shwap.Getter, ds datastore.Batching) *light.ShareAvailability {
					return light.NewShareAvailability(
						getter,
						ds,
						light.WithSampleAmount(cfg.LightAvailability.SampleAmount),
					)
				},
				fx.OnStop(func(ctx context.Context, la *light.ShareAvailability) error {
					return la.Close(ctx)
				}),
			)),
			fx.Provide(func(avail *light.ShareAvailability) share.Availability {
				return avail
			}),
		)
	case node.Bridge, node.Full:
		return fx.Options(
			fx.Provide(full.NewShareAvailability),
			fx.Provide(func(avail *full.ShareAvailability) share.Availability {
				return avail
			}),
		)
	default:
		panic("invalid node type")
	}
}
