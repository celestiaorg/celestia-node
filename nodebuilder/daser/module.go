package daser

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	fraudServ "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type, cfg *das.Config) fx.Option {
	baseComponents := fx.Options(
		fx.Provide(
			fx.Annotate(
				// provide DASer configuration in here
				// example:
				//
				//	return []das.Option{
				//			das.WithParamBackgroundStoreInterval(10*100*time.Nanosecond),
				//			das.WithParamConcurrencyLimit(32)
				//	}
				func() []das.Option {
					return []das.Option{
						das.WithParamSamplingRange(int(300)),
						das.WithParamConcurrencyLimit(int(32)),
					}
				},
				fx.ResultTags(`name:"DASOptions"`),
			),
		),
	)

	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"daser",
			baseComponents,
			fx.Provide(
				fx.Annotate(
					NewDASer,
					fx.ParamTags(``, ``, ``, ``, ``, `name:"DASOptions"`),
					fx.OnStart(func(ctx context.Context, lc fx.Lifecycle, fservice fraudServ.Module, das *das.DASer) error {
						lifecycleCtx := fxutil.WithLifecycle(ctx, lc)
						return fraudServ.Lifecycle(ctx, lifecycleCtx, fraud.BadEncoding, fservice, das.Start, das.Stop)
					}),
					fx.OnStop(func(ctx context.Context, das *das.DASer) error {
						return das.Stop(ctx)
					}),
				),
			),
		)
	case node.Bridge:
		return fx.Options()
	default:
		panic("invalid node type")
	}
}
