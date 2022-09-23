package state

import (
	"context"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	fraudbuilder "github.com/celestiaorg/celestia-node/nodebuilder/fraud"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var log = logging.Logger("state-module")

// Module provides all components necessary to construct the
// state service.
func Module(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(Keyring),
		fx.Provide(fx.Annotate(CoreAccessor,
			fx.OnStart(func(ctx context.Context, lc fx.Lifecycle, fservice fraud.Service, serv Service) error {
				lifecycleCtx := fxutil.WithLifecycle(ctx, lc)
				return fraudbuilder.Lifecycle(ctx, lifecycleCtx, fraud.BadEncoding, fservice,
					serv.Start, serv.Stop)
			}),
			fx.OnStop(func(ctx context.Context, serv Service) error {
				return serv.Stop(ctx)
			}),
		)),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"state",
			baseComponents,
		)
	default:
		panic("invalid node type")
	}
}
