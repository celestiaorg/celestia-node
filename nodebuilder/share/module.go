package share

import (
	"context"

	"github.com/celestiaorg/celestia-node/share"

	"github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *Config, options ...fx.Option) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Options(options...),
		fx.Invoke(share.EnsureEmptySquareExists),
		fx.Provide(Discovery(*cfg)),
		fx.Provide(NewModule),
	)

	switch tp {
	case node.Light:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(fx.Annotate(
				light.NewLightAvailability,
				fx.OnStart(func(ctx context.Context, avail *light.ShareAvailability) error {
					return avail.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, avail *light.ShareAvailability) error {
					return avail.Stop(ctx)
				}),
			)),
			// CacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a CacheAvailability but the constructor returns a share.Availability
			fx.Provide(CacheAvailability[*light.ShareAvailability]),
		)
	case node.Bridge, node.Full:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(fx.Annotate(
				full.NewShareAvailability,
				fx.OnStart(func(ctx context.Context, avail *full.ShareAvailability) error {
					return avail.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, avail *full.ShareAvailability) error {
					return avail.Stop(ctx)
				}),
			)),
			// CacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a CacheAvailability but the constructor returns a share.Availability
			fx.Provide(CacheAvailability[*full.ShareAvailability]),
		)
	default:
		panic("invalid node type")
	}
}
