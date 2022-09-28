package share

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
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
		fx.Provide(fx.Annotate(
			NewModule,
			fx.OnStart(func(ctx context.Context, module Module) error {
				return module.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, module Module) error {
				return module.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(fx.Annotate(
				share.NewLightAvailability,
				fx.OnStart(func(ctx context.Context, avail *share.LightAvailability) error {
					return avail.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, avail *share.LightAvailability) error {
					return avail.Stop(ctx)
				}),
			)),
			// CacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a CacheAvailability but the constructor returns a share.Availability
			fx.Provide(CacheAvailability[*share.LightAvailability]),
		)
	case node.Bridge, node.Full:
		return fx.Module(
			"share",
			baseComponents,
			fx.Provide(fx.Annotate(
				share.NewFullAvailability,
				fx.OnStart(func(ctx context.Context, avail *share.FullAvailability) error {
					return avail.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, avail *share.FullAvailability) error {
					return avail.Stop(ctx)
				}),
			)),
			// CacheAvailability's lifecycle continues to use a fx hook,
			// since the LC requires a CacheAvailability but the constructor returns a share.Availability
			fx.Provide(CacheAvailability[*share.FullAvailability]),
		)
	default:
		panic("invalid node type")
	}
}
