package share

import (
	"bytes"
	"context"
	"encoding/hex"
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
	"github.com/celestiaorg/celestia-node/store/file"
	libshare "github.com/celestiaorg/go-square/v3/share"
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
	case node.Bridge, node.Full, node.Pin:
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
	case node.Full, node.Bridge, node.Pin:
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
				cfg.ShrexClient.WithNetworkID(network.String())
				return shrex.NewClient(cfg.ShrexClient, host)
			},
		),

		// shrex-getter
		fx.Provide(fx.Annotate(
			func(
				client *shrex.Client,
				managers map[string]*peers.Manager,
			) *shrex_getter.Getter {
				return shrex_getter.NewGetter(
					client,
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
	case node.Full, node.Pin:
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
				cfg.ShrexServer.WithNetworkID(network.String())
				return shrex.NewServer(cfg.ShrexServer, host, store)
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
			func(path node.StorePath, ds datastore.Batching) (*store.Store, error) {
				if cfg.NamespaceID != "" {
					filter, err := compileFilter(cfg.NamespaceID)
					if err != nil {
						return nil, fmt.Errorf("nodebuilder/share: compiling namespace filter: %w", err)
					}
					cfg.EDSStoreParams.NamespaceFilter = filter

					cfg.EDSStoreParams.NamespaceID, err = parseNamespaceID(cfg.NamespaceID)
					if err != nil {
						return nil, fmt.Errorf("nodebuilder/share: parsing namespace id: %w", err)
					}
				}
				cfg.EDSStoreParams.Datastore = ds
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
	case node.Bridge, node.Full, node.Pin:
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

func compileFilter(id string) (file.NamespaceFilter, error) {
	if id == "" {
		return nil, nil
	}
	ns, err := parseNamespaceID(id)
	if err != nil {
		return nil, err
	}
	return func(other libshare.Namespace) bool {
		return bytes.Equal(other.Bytes(), ns.Bytes())
	}, nil
}

// parseNamespaceID parses a namespace ID from a hex string.
// It supports both:
//   - Full 58-character hex (29 bytes): version byte + 28 byte ID
//   - Short hex (up to 20 characters / 10 bytes): user portion only, left-padded as v0
func parseNamespaceID(id string) (libshare.Namespace, error) {
	decoded, err := hex.DecodeString(id)
	if err != nil {
		return libshare.Namespace{}, fmt.Errorf("invalid hex: %w", err)
	}

	// Full namespace (29 bytes = 1 version + 28 ID)
	if len(decoded) == libshare.NamespaceSize {
		return libshare.NewNamespaceFromBytes(decoded)
	}

	// Short format: treat as user portion of a v0 namespace (max 10 bytes)
	if len(decoded) <= 10 {
		return libshare.NewV0Namespace(decoded)
	}

	return libshare.Namespace{}, fmt.Errorf(
		"invalid namespace length: got %d bytes, expected %d (full) or <= 10 (short v0)",
		len(decoded), libshare.NamespaceSize,
	)
}
