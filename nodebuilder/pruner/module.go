package pruner

import (
	"context"

	"github.com/ipfs/go-datastore"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/archival"
	"github.com/celestiaorg/celestia-node/pruner/full"
	"github.com/celestiaorg/celestia-node/pruner/light"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(cfg),
		availWindow(tp, cfg.EnableService),
	)

	prunerService := fx.Options(
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
	case node.Light:
		if cfg.EnableService {
			return fx.Module("prune",
				baseComponents,
				prunerService,
				fx.Provide(light.NewPruner),
			)
		}
		// We do not trigger DetectPreviousRun for Light nodes, to allow them to disable pruning at wish.
		// They are not expected to store a samples outside the sampling window and so partially pruned is
		// not a concern.
		return fx.Module("prune",
			baseComponents,
		)
	case node.Full:
		if cfg.EnableService {
			return fx.Module("prune",
				baseComponents,
				prunerService,
				fxutil.ProvideAs(full.NewPruner, new(pruner.Pruner)),
			)
		}
		return fx.Module("prune",
			baseComponents,
			fx.Invoke(func(ctx context.Context, ds datastore.Batching) error {
				return pruner.DetectPreviousRun(ctx, ds)
			}),
		)
	case node.Bridge:
		if cfg.EnableService {
			return fx.Module("prune",
				baseComponents,
				prunerService,
				fxutil.ProvideAs(full.NewPruner, new(pruner.Pruner)),
				fx.Provide(func(window pruner.AvailabilityWindow) []core.Option {
					return []core.Option{core.WithAvailabilityWindow(window)}
				}),
			)
		}
		return fx.Module("prune",
			baseComponents,
			fx.Invoke(func(ctx context.Context, ds datastore.Batching) error {
				return pruner.DetectPreviousRun(ctx, ds)
			}),
			fx.Provide(func() []core.Option {
				return []core.Option{}
			}),
		)
	default:
		panic("unknown node type")
	}
}

func availWindow(tp node.Type, pruneEnabled bool) fx.Option {
	switch tp {
	case node.Light:
		// light nodes are still subject to sampling within window
		// even if pruning is not enabled.
		return fx.Provide(func() pruner.AvailabilityWindow {
			return light.Window
		})
	case node.Full, node.Bridge:
		return fx.Provide(func() pruner.AvailabilityWindow {
			if pruneEnabled {
				return full.Window
			}
			return archival.Window
		})
	default:
		panic("unknown node type")
	}
}
