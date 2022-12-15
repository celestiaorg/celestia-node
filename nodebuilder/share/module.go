package share

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/p2p"
)

func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Options(options...),
		fx.Invoke(share.EnsureEmptySquareExists),
		fx.Provide(discovery(*cfg)),
		fx.Provide(newModule),
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
				func(host host.Host, store *eds.Store, path node.StorePath) *p2p.Server {
					return p2p.NewServer(host, store, string(path))
				},
				fx.OnStart(func(ctx context.Context, server *p2p.Server) error {
					return server.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, server *p2p.Server) error {
					return server.Stop(ctx)
				}),
			)),
			// Bridge Nodes need a client as well, for requests over FullAvailability
			fx.Provide(
				func(host host.Host, path node.StorePath) *p2p.Client {
					return p2p.NewClient(host, string(path))
				},
			),
			fx.Provide(fx.Annotate(
				func(path node.StorePath, ds datastore.Batching) (*eds.Store, error) {
					return eds.NewStore(string(path), ds)
				},
				fx.OnStart(func(ctx context.Context, eds *eds.Store) error {
					return eds.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, eds *eds.Store) error {
					return eds.Stop(ctx)
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
			// cacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a cacheAvailability but the constructor returns a share.Availability
			fx.Provide(cacheAvailability[*full.ShareAvailability]),
		)
	default:
		panic("invalid node type")
	}
}
