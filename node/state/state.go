package state

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/key"
	"github.com/celestiaorg/celestia-node/node/services"
	"github.com/celestiaorg/celestia-node/service/state"
)

var log = logging.Logger("state-access-constructor")

// Module provides all components necessary to construct the
// state service.
func Module(coreCfg core.Config, keyCfg key.Config) fx.Option {
	return fx.Module(
		"state",
		fx.Provide(Keyring(keyCfg)),
		fx.Provide(fx.Annotate(CoreAccessor(coreCfg.IP, coreCfg.RPCPort, coreCfg.GRPCPort),
			fx.OnStart(func(ctx context.Context, accessor state.Accessor) error {
				return accessor.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, accessor state.Accessor) error {
				return accessor.Stop(ctx)
			}),
		)),
		fx.Provide(fx.Annotate(state.NewService,
			fx.OnStart(func(ctx context.Context, lc fx.Lifecycle, fservice fraud.Service, serv *state.Service) error {
				lifecycleCtx := fxutil.WithLifecycle(ctx, lc)
				return services.FraudLifecycle(ctx, lifecycleCtx, fraud.BadEncoding, fservice, serv.Start, serv.Stop)
			}),
			fx.OnStop(func(ctx context.Context, serv *state.Service) error {
				return serv.Stop(ctx)
			}),
		)),
	)
}