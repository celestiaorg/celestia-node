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
	"github.com/celestiaorg/celestia-node/share/p2p/shrexeds"
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
		fx.Invoke(share.EnsureEmptySquareExists),
		fxutil.ProvideAs(getters.NewIPLDGetter, new(share.Getter)),
	)

	switch tp {
	case node.Light:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(fx.Annotate(
				light.NewShareAvailability,
				fx.OnStart(func(ctx context.Context, avail *light.ShareAvailability) error {
					return avail.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, avail *light.ShareAvailability) error {
					return avail.Stop(ctx)
				}),
			)),
			// cacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a cacheAvailability but the constructor returns a share.Availability
			fx.Provide(cacheAvailability[*light.ShareAvailability]),
		)
	case node.Bridge, node.Full:
		return fx.Module(
			"share",
			baseComponents,
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
			fx.Provide(fx.Annotate(
				func(ctx context.Context, h host.Host, network modp2p.Network) (*shrexsub.PubSub, error) {
					return shrexsub.NewPubSub(
						ctx,
						h,
						string(network),
					)
				},
				fx.OnStart(func(ctx context.Context, pubsub *shrexsub.PubSub) error {
					return pubsub.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, pubsub *shrexsub.PubSub) error {
					return pubsub.Stop(ctx)
				}),
			)),
			// cacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a cacheAvailability but the constructor returns a share.Availability
			fx.Provide(cacheAvailability[*full.ShareAvailability]),
		)
	default:
		panic("invalid node type")
	}
}
