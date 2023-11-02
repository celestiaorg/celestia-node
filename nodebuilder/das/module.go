package das

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/pruner"
)

func ConstructModule(tp node.Type, cfg *Config, prunerCfg *pruner.Config) fx.Option {
	var err error
	// do not validate daser config for bridge node as it
	// does not need it
	if tp != node.Bridge {
		err = cfg.Validate()
	}

	var daserOptions fx.Option
	if prunerCfg.PruningEnabled {
		daserOptions = fx.Provide(
			func(c Config, pruner *pruner.StoragePruner) []das.Option {
				options := []das.Option{
					das.WithSamplingRange(c.SamplingRange),
					das.WithConcurrencyLimit(c.ConcurrencyLimit),
					das.WithBackgroundStoreInterval(c.BackgroundStoreInterval),
					das.WithSampleFrom(c.SampleFrom),
					das.WithSampleTimeout(c.SampleTimeout),
					das.WithRecencyWindow(prunerCfg.RecencyWindow),
				}

				if prunerCfg.PruningEnabled {
					options = append(options, das.WithStoragePruner(pruner))
				}
				return options
			},
		)
	} else {
		daserOptions = fx.Provide(
			func(c Config) []das.Option {
				return []das.Option{
					das.WithSamplingRange(c.SamplingRange),
					das.WithConcurrencyLimit(c.ConcurrencyLimit),
					das.WithBackgroundStoreInterval(c.BackgroundStoreInterval),
					das.WithSampleFrom(c.SampleFrom),
					das.WithSampleTimeout(c.SampleTimeout),
					das.WithRecencyWindow(prunerCfg.RecencyWindow),
				}
			},
		)
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(err),
		daserOptions,
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"das",
			baseComponents,
			fx.Provide(fx.Annotate(
				newDASer,
				fx.OnStart(func(
					ctx context.Context, breaker *modfraud.ServiceBreaker[*das.DASer, *header.ExtendedHeader],
				) error {
					return breaker.Start(ctx)
				}),
				fx.OnStop(func(
					ctx context.Context, breaker *modfraud.ServiceBreaker[*das.DASer, *header.ExtendedHeader],
				) error {
					return breaker.Stop(ctx)
				}),
			)),
			// Module is needed for the RPC handler
			fx.Provide(func(das *das.DASer) Module {
				return das
			}),
		)
	case node.Bridge:
		return fx.Module(
			"das",
			baseComponents,
			fx.Provide(newDaserStub),
		)
	default:
		panic("invalid node type")
	}
}
