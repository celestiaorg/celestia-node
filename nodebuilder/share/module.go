package share

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/peers"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrex_getter"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/shrexsub"
	"github.com/celestiaorg/celestia-node/store"
)

func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	err := cfg.Validate(tp)
	if err != nil {
		return fx.Error(fmt.Errorf("nodebuilder/share: validate config: %w", err))
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Options(options...),
		fx.Provide(newShareModule),
		availabilityComponents(tp, cfg),
		shrexComponents(tp, cfg),
		bitswapComponents(tp, cfg),
		peerManagementComponents(tp, cfg),
	)

	switch tp {
	case node.Bridge, node.Full:
		return fx.Module(
			"share",
			baseComponents,
			edsStoreComponents(cfg),
			fx.Provide(bridgeAndFullGetter),
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

func bitswapComponents(tp node.Type, cfg *Config) fx.Option {
	opts := fx.Options(
		fx.Provide(dataExchange),
		fx.Provide(bitswapGetter),
	)
	switch tp {
	case node.Light:
		return fx.Options(
			opts,
			fx.Provide(
				fx.Annotate(
					blockstoreFromDatastore,
					fx.As(fx.Self()),
					fx.As(new(blockstore.Blockstore)),
				),
			),
		)
	case node.Full, node.Bridge:
		return fx.Options(
			opts,
			fx.Provide(
				fx.Annotate(
					func(store *store.Store) (*bitswap.BlockstoreWithMetrics, error) {
						return blockstoreFromEDSStore(store, int(cfg.BlockStoreCacheSize))
					},
					fx.As(fx.Self()),
					fx.As(new(blockstore.Blockstore)),
				),
			),
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
			func(host host.Host, network modp2p.Network) (*shrex.Client, error) {
				cfg.ShrExNDParams.WithNetworkID(network.String())
				return shrex.NewClient(cfg.ShrExNDParams, host)
			},
		),

		// shrex-getter
		fx.Provide(fx.Annotate(
			func(
				ndClient *shrex.Client,
				managers map[string]*peers.Manager,
			) *shrex_getter.Getter {
				return shrex_getter.NewGetter(
					ndClient,
					managers[fullNodesTag],
					managers[archivalNodesTag],
					availability.RequestWindow,
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
		fx.Invoke(func(_ *shrex.Server) {}),
		fx.Provide(fx.Annotate(
			func(
				host host.Host,
				store *store.Store,
				network modp2p.Network,
			) (*shrex.Server, error) {
				cfg.ShrExNDParams.WithNetworkID(network.String())
				return shrex.NewServer(cfg.ShrExNDParams, host, store, shrex.SupportedProtocols())
			},
			fx.OnStart(func(ctx context.Context, server *shrex.Server) error {
				return server.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, server *shrex.Server) error {
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
				func(getter shwap.Getter, ds datastore.Batching, bs blockstore.Blockstore) *light.ShareAvailability {
					return light.NewShareAvailability(
						getter,
						ds,
						bs,
						light.WithSampleAmount(cfg.LightAvailability.SampleAmount),
					)
				},
				fx.As(fx.Self()),
				fx.As(new(share.Availability)),
				fx.OnStop(func(ctx context.Context, la *light.ShareAvailability) error {
					return la.Close(ctx)
				}),
			)),
		)
	case node.Bridge, node.Full:
		return fx.Options(
			fx.Provide(func(
				s *store.Store,
				getter shwap.Getter,
				opts []full.Option,
			) *full.ShareAvailability {
				return full.NewShareAvailability(s, getter, opts...)
			}),
			fx.Provide(func(avail *full.ShareAvailability) share.Availability {
				return avail
			}),
		)
	default:
		panic("invalid node type")
	}
}
