package daser

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	fraudServ "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func ConstructModule(tp node.Type) fx.Option {
	switch tp {
	case node.Light, node.Full:
		return fx.Module(
			"daser",
			fx.Provide(fx.Annotate(
				NewDASer,
				fx.OnStart(func(startCtx, ctx context.Context, fservice fraudServ.Module, das *das.DASer) error {
					return fraudServ.Lifecycle(startCtx, ctx, fraud.BadEncoding, fservice,
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
