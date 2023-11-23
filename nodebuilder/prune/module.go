package prune

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/archival"
	"github.com/celestiaorg/celestia-node/pruner/light"
)

func ConstructModule(tp node.Type) fx.Option {
	baseComponents := fx.Options(
		fx.Provide(fx.Annotate(
			pruner.NewPruner,
			fx.OnStart(func(ctx context.Context, p *pruner.Pruner) error {
				return p.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, p *pruner.Pruner) error {
				return p.Stop(ctx)
			}),
		)),
		fx.Provide(
			func(factory pruner.Factory) pruner.AvailabilityWindow {
				return factory
			}),
	)

	switch tp {
	case node.Full, node.Bridge:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func() pruner.Factory {
				return archival.NewPruner()
			}),
		)
	case node.Light:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func() pruner.Factory {
				return light.NewPruner()
			}),
		)
	default:
		panic("unknown node type")
	}
}
