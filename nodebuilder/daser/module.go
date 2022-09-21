package daser

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	fraudbuilder "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func Module(tp node.Type) fx.Option {
	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"daser",
			fx.Provide(fx.Annotate(
				NewDASer,
				fx.OnStart(func(ctx context.Context, lc fx.Lifecycle, fservice fraud.Service, das *das.DASer) error {
					lifecycleCtx := fxutil.WithLifecycle(ctx, lc)
					return fraudbuilder.Lifecycle(ctx, lifecycleCtx, fraud.BadEncoding, fservice,
						das.Start, das.Stop)
				}),
				fx.OnStop(func(ctx context.Context, das *das.DASer) error {
					return das.Stop(ctx)
				}),
			)),
		)
	case node.Bridge:
		return fx.Options()
	default:
		panic("invalid node type")
	}
}
