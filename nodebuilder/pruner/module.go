package pruner

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
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

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
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
		fx.Supply(cfg),
		// TODO @renaynay: move this to share module construction
		fx.Supply(modshare.Window(availability.StorageWindow)),
		advertiseArchival(tp, cfg),
		prunerService,
	)

	switch tp {
	case node.Light:
		// LNs enforce pruning by default
		return fx.Module("prune",
			baseComponents,
			// TODO(@walldiss @renaynay): remove conversion after Availability and Pruner interfaces are merged
			//  note this provide exists in pruner module to avoid cyclical imports
			fx.Provide(func(la *light.ShareAvailability) pruner.Pruner { return la }),
		)
	case node.Full:
		fullAvailOpts := make([]fullavail.Option, 0)

		if !cfg.EnableService {
			// populate archival mode opts
			fullAvailOpts = []fullavail.Option{fullavail.WithArchivalMode()}
		}

		return fx.Module("prune",
			baseComponents,
			fx.Supply(fullAvailOpts),
			fx.Provide(func(fa *fullavail.ShareAvailability) pruner.Pruner { return fa }),
			convertToPruned(),
		)
	case node.Bridge:
		coreOpts := make([]core.Option, 0)
		fullAvailOpts := make([]fullavail.Option, 0)

		if !cfg.EnableService {
			// populate archival mode opts
			coreOpts = []core.Option{core.WithArchivalMode()}
			fullAvailOpts = []fullavail.Option{fullavail.WithArchivalMode()}
		}

		return fx.Module("prune",
			baseComponents,
			fx.Provide(func(fa *fullavail.ShareAvailability) pruner.Pruner { return fa }),
			fx.Supply(coreOpts),
			fx.Supply(fullAvailOpts),
			convertToPruned(),
		)
	default:
		panic("unknown node type")
	}
}

func advertiseArchival(tp node.Type, pruneCfg *Config) fx.Option {
	if (tp == node.Full || tp == node.Bridge) && !pruneCfg.EnableService {
		return fx.Supply(discovery.WithAdvertise())
	}
	return fx.Provide(func() discovery.Option {
		var opt discovery.Option
		return opt
	})
}

// convertToPruned checks if the node is being converted to an archival node
// to a pruned node.
func convertToPruned() fx.Option {
	return fx.Invoke(func(
		ctx context.Context,
		cfg *Config,
		ds datastore.Batching,
		p *pruner.Service,
	) error {
		ds = namespace.Wrap(ds, storePrefix)

		lastPrunedHeight, err := p.LastPruned(ctx)
		if err != nil {
			return err
		}

		err = detectFirstRun(ctx, cfg, ds, lastPrunedHeight)
		if err != nil {
			return err
		}

		convert, err := convertFromArchivalToPruned(ctx, cfg, ds)
		if err != nil {
			return err
		}

		if convert {
			return p.ClearCheckpoint(ctx)
		}

		return nil
	})
}
