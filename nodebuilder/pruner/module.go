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
	"github.com/celestiaorg/celestia-node/share/availability"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	baseComponents := fx.Options(
		fx.Supply(cfg),
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
		// enforce pruning by default
		return fx.Module("prune",
			baseComponents,
			prunerService,
			fxutil.ProvideAs(light.NewPruner, new(pruner.Pruner)),
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
			prunerService,
			fxutil.ProvideAs(archival.NewPruner, new(pruner.Pruner)),
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
				fx.Provide(func(window availability.Window) []core.Option {
					return []core.Option{
						core.WithAvailabilityWindow(window.Duration()),
						core.WithPruningEnabled(),
					}
				}),
			)
		}
		return fx.Module("prune",
			baseComponents,
			prunerService,
			fxutil.ProvideAs(archival.NewPruner, new(pruner.Pruner)),
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
