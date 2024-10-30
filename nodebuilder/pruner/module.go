package pruner

import (
	"context"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/pruner/full"
	"github.com/celestiaorg/celestia-node/pruner/light"
	"github.com/celestiaorg/celestia-node/share/availability"
)

var log = logging.Logger("module/pruner")

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
				fx.Provide(func(window availability.Window) []core.Option {
					return []core.Option{core.WithAvailabilityWindow(window.Duration())}
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
		return fx.Provide(func() availability.Window {
			return availability.Window(availability.StorageWindow)
		})
	case node.Full, node.Bridge:
		return fx.Provide(func() availability.Window {
			if pruneEnabled {
				return availability.Window(availability.StorageWindow)
			}
			// implicitly disable pruning by setting the window to 0
			return availability.Window(time.Duration(0))
		})
	default:
		panic("unknown node type")
	}
}
