package prune

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/archival"
	"github.com/celestiaorg/celestia-node/pruner/light"
)

func ConstructModule(tp node.Type) fx.Option {
	fmt.Print("\n\n\n\nconstructing pruning module\n\n\n\n")

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
	case node.Full, node.Bridge:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func() pruner.Pruner {
				return archival.NewPruner()
			}),
			fx.Supply(archival.Window),
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
