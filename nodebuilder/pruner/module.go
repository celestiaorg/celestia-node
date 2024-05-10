package pruner

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/archival"
	"github.com/celestiaorg/celestia-node/pruner/full"
	"github.com/celestiaorg/celestia-node/pruner/light"
	"github.com/celestiaorg/celestia-node/share/eds"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	if !cfg.EnableService {
		switch tp {
		case node.Light:
			// light nodes are still subject to sampling within window
			// even if pruning is not enabled.
			return fx.Options(
				fx.Supply(light.Window),
			)
		case node.Full:
			return fx.Options(
				fx.Supply(archival.Window),
			)
		case node.Bridge:
			return fx.Options(
				fx.Supply(archival.Window),
				fx.Provide(func() []core.Option {
					return []core.Option{}
				}),
			)
		default:
			panic("unknown node type")
		}
	}

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
		fx.Invoke(func(_ *pruner.Service) {}),
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
			fx.Provide(func(window pruner.AvailabilityWindow) []core.Option {
				return []core.Option{core.WithAvailabilityWindow(window)}
			}),
		)
	case node.Light:
		fx.Provide(light.NewPruner)
		return fx.Module("prune",
			fx.Supply(light.Window),
		)
	default:
		panic("unknown node type")
	}
}
