package prune

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/full"
	"github.com/celestiaorg/celestia-node/pruner/light"
	"github.com/celestiaorg/celestia-node/share/eds"
)

func ConstructModule(tp node.Type) fx.Option {

	baseComponents := fx.Options(
		fx.Provide(fx.Annotate(
			newPrunerService,
			fx.OnStart(func(ctx context.Context, p *pruner.Service) error {
				return p.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, p *pruner.Service) error {
				return p.Stop(ctx)
			}),
		)),
		// This is necessary to invoke the pruner service as independent thanks to a
		// quirk in FX.
		fx.Invoke(func(p *pruner.Service) {}),
	)

	switch tp {
	case node.Full:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func(store *eds.Store) pruner.Pruner {
				return full.NewPruner(store)
			}),
			fx.Supply(full.Window),
		)
	case node.Bridge:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func(store *eds.Store) pruner.Pruner {
				return full.NewPruner(store)
			}),
			fx.Supply(full.Window),
			//fx.Provide(func() pruner.Pruner {
			//	return archival.NewPruner()
			//}),
			//fx.Supply(archival.Window),
		)
	case node.Light:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func() pruner.Pruner {
				return light.NewPruner()
			}),
			fx.Supply(light.Window),
		)
	default:
		panic("unknown node type")
	}
}
