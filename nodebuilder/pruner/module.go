package pruner

import (
	"context"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modshare "github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/pruner"
	"github.com/celestiaorg/celestia-node/share/availability"
	fullavail "github.com/celestiaorg/celestia-node/share/availability/full"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
)

var log = logging.Logger("module/pruner")

func ConstructModule(tp node.Type) fx.Option {
	cfg := DefaultConfig()
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

	baseComponents := fx.Options(
		// supply the default config, which can only be overridden by
		// passing the `--archival` flag
		fx.Supply(cfg),
		fx.Provide(func(cfg *Config) node.ArchivalMode {
			return node.ArchivalMode(!cfg.EnableService)
		}),
		// TODO @renaynay: move this to share module construction
		advertiseArchival(),
		prunerService,
	)

	switch tp {
	case node.Light:
		// LNs enforce pruning by default
		return fx.Module("prune",
			baseComponents,
			fx.Supply(modshare.Window(availability.SamplingWindow)),
			// TODO(@walldiss @renaynay): remove conversion after Availability and Pruner interfaces are merged
			//  note this provide exists in pruner module to avoid cyclical imports
			fx.Provide(func(la *light.ShareAvailability) pruner.Pruner { return la }),
		)
	case node.Bridge:
		return fx.Module("prune",
			baseComponents,
			fx.Provide(func(cfg *Config) ([]core.Option, []fullavail.Option) {
				if cfg.EnableService {
					return make([]core.Option, 0), make([]fullavail.Option, 0)
				}
				return []core.Option{core.WithArchivalMode()}, []fullavail.Option{fullavail.WithArchivalMode()}
			}),
			fx.Provide(func(fa *fullavail.ShareAvailability) pruner.Pruner { return fa }),
			fx.Supply(modshare.Window(availability.StorageWindow)),
			fx.Invoke(convertToPruned),
		)
	default:
		panic("unknown node type")
	}
}

func advertiseArchival() fx.Option {
	return fx.Provide(func(tp node.Type, pruneCfg *Config) discovery.Option {
		if tp == node.Bridge && !pruneCfg.EnableService {
			return discovery.WithAdvertise()
		}
		var opt discovery.Option
		return opt
	})
}

// convertToPruned checks if the node is being converted to an archival node
// to a pruned node.
func convertToPruned(
	lc fx.Lifecycle,
	cfg *Config,
	ds datastore.Batching,
	p *pruner.Service,
) error {
	convertFn := func(ctx context.Context) error {
		lastPrunedHeight, err := p.LastPruned(ctx)
		if err != nil {
			return err
		}

		err = detectFirstRun(ctx, cfg, ds, lastPrunedHeight)
		if err != nil {
			return err
		}

		isArchival := !cfg.EnableService
		convert, err := fullavail.ConvertFromArchivalToPruned(ctx, ds, isArchival)
		if err != nil {
			return err
		}

		// if we convert the node from archival to pruned, we need to reset the checkpoint
		// to ensure the node goes back and deletes *all* blocks older than the
		// availability window, as archival "pruning" only trims the .q4 file,
		// but retains the ODS.
		if convert {
			return p.ResetCheckpoint(ctx)
		}

		return nil
	}

	lc.Append(fx.StartHook(convertFn))
	return nil
}
