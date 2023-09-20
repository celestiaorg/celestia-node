package pruner

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(*cfg),
	)

	var pruner fx.Option
	if cfg.PruningEnabled {
		pruner = fx.Options(
			fx.Provide(fx.Annotate(
				NewStoragePruner,
				fx.OnStart(func(ctx context.Context, pruner *StoragePruner) error {
					return pruner.Start(ctx)
				}),
				fx.OnStop(func(ctx context.Context, pruner *StoragePruner) error {
					return pruner.Stop(ctx)
				}),
			)),
		)
	}

	switch tp {
	case node.Light:
		return fx.Module(
			"pruner",
			baseComponents,
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"pruner",
			baseComponents,
			pruner,
		)
	default:
		panic("invalid node type")
	}
}
