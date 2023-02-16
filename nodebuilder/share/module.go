package share

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/getters"
	"github.com/celestiaorg/celestia-node/share/p2p/peers"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexnd"
	"github.com/celestiaorg/celestia-node/share/p2p/shrexsub"
)

func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Options(options...),
		fx.Provide(discovery(*cfg)),
		fx.Provide(newModule),
		// TODO: Configure for light nodes
		fx.Provide(
			func(host host.Host, network modp2p.Network) (*shrexnd.Client, error) {
				return shrexnd.NewClient(host, shrexnd.WithProtocolSuffix(string(network)))
			},
		),
	)

	switch tp {
	case node.Light:
		return fx.Module(
			"share",
			baseComponents,
			fx.Invoke(share.EnsureEmptySquareExists),
			// shrexsub broadcaster stub for daser
			fx.Provide(func() shrexsub.BroadcastFn {
				return func(context.Context, share.DataHash) error {
					return nil
				}
			}),
			fxutil.ProvideAs(getters.NewIPLDGetter, new(share.Getter)),
			fx.Provide(fx.Annotate(light.NewShareAvailability)),
			// cacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a cacheAvailability but the constructor returns a share.Availability
			fx.Provide(cacheAvailability[*light.ShareAvailability]),
		)
	case node.Bridge, node.Full:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(getters.NewIPLDGetter),
			fx.Invoke(func(edsSrv *shrexeds.Server, ndSrc *shrexnd.Server) {}),
			fx.Provide(fx.Annotate(
				func(host host.Host, store *eds.Store, network modp2p.Network) (*shrexeds.Server, error) {
					return shrexeds.NewServer(host, store, shrexeds.WithProtocolSuffix(string(network)))
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
					store *eds.Store,
					getter *getters.IPLDGetter,
					network modp2p.Network,
				) (*shrexnd.Server, error) {
					return shrexnd.NewServer(host, store, getter, shrexnd.WithProtocolSuffix(string(network)))
				},
				fx.OnStart(func(ctx context.Context, server *shrexnd.Server) error {
					return server.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, server *shrexnd.Server) error {
					return server.Stop(ctx)
				}),
			)),
			// Bridge Nodes need a client as well, for requests over FullAvailability
			fx.Provide(
				func(host host.Host, network modp2p.Network) (*shrexeds.Client, error) {
					return shrexeds.NewClient(host, shrexeds.WithProtocolSuffix(string(network)))
				},
			),
			fx.Provide(fx.Annotate(
				func(path node.StorePath, ds datastore.Batching) (*eds.Store, error) {
					return eds.NewStore(string(path), ds)
				},
				fx.OnStart(func(ctx context.Context, store *eds.Store) error {
					err := store.Start(ctx)
					if err != nil {
						return err
					}

					return ensureEmptyCARExists(ctx, store)
				}),
				fx.OnStop(func(ctx context.Context, store *eds.Store) error {
					return store.Stop(ctx)
				}),
			)),
			fx.Provide(fx.Annotate(
				full.NewShareAvailability,
				fx.OnStart(func(ctx context.Context, avail *full.ShareAvailability) error {
					return avail.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, avail *full.ShareAvailability) error {
					return avail.Stop(ctx)
				}),
			)),
			fx.Provide(
				func(ctx context.Context, h host.Host, network modp2p.Network) (*shrexsub.PubSub, error) {
					return shrexsub.NewPubSub(
						ctx,
						h,
						string(network),
					)
				},
			),
			// cacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a cacheAvailability but the constructor returns a share.Availability
			fx.Provide(cacheAvailability[*full.ShareAvailability]),
			fx.Provide(fx.Annotate(
				getters.NewShrexGetter,
				fx.OnStart(func(ctx context.Context, getter *getters.ShrexGetter) error {
					return getter.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, getter *getters.ShrexGetter) error {
					return getter.Stop(ctx)
				}),
			)),
			fx.Provide(func(shrexSub *shrexsub.PubSub) shrexsub.BroadcastFn {
				return shrexSub.Broadcast
			}),
			fx.Provide(peers.NewManager),
			fx.Provide(fullGetter),
		)
	default:
		panic("invalid node type")
	}
}
