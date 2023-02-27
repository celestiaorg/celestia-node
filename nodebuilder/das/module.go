package das

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	fraudServ "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	var err error
	// do not validate daser config for bridge node as it
	// does not need it
	if tp != node.Bridge {
		err = cfg.Validate()
	}

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(err),
		fx.Provide(
			func(c Config) []das.Option {
				return []das.Option{
					das.WithSamplingRange(c.SamplingRange),
					das.WithConcurrencyLimit(c.ConcurrencyLimit),
					das.WithBackgroundStoreInterval(c.BackgroundStoreInterval),
					das.WithSampleFrom(c.SampleFrom),
					das.WithSampleTimeout(c.SampleTimeout),
				}
			},
		),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"daser",
			baseComponents,
			fx.Provide(fx.Annotate(
				newDASer,
				fx.OnStart(func(startCtx, ctx context.Context, fservice fraud.Service, das *das.DASer) error {
					return fraudServ.Lifecycle(startCtx, ctx, byzantine.BadEncoding, fservice,
						das.Start, das.Stop)
				}),
				fx.OnStop(func(ctx context.Context, das *das.DASer) error {
					return das.Stop(ctx)
				}),
			)),
			// Module is needed for the RPC handler
			fx.Provide(func(das *das.DASer) Module {
				return das
			}),
		)
	case node.Bridge:
		return fx.Module(
			"daser",
			baseComponents,
			fx.Provide(newDaserStub),
		)
	default:
		panic("invalid node type")
	}
}
