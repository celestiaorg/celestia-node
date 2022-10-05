package state

import (
	"context"

	"go.uber.org/fx"

	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	fraudServ "github.com/celestiaorg/celestia-node/nodebuilder/fraud"

	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/state"
)

// CoreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func CoreAccessor(
	lc fx.Lifecycle,
	fService fraudServ.Module,
	corecfg core.Config,
	signer *apptypes.KeyringSigner,
	sync *sync.Syncer,
) (Module, *state.CoreAccessor) {
	service := state.NewCoreAccessor(signer, sync, corecfg.IP, corecfg.RPCPort, corecfg.GRPCPort)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lifecycleCtx := fxutil.WithLifecycle(ctx, lc)
			return fraudServ.Lifecycle(ctx, lifecycleCtx, fraud.BadEncoding, fService, service.Start, service.Stop)
		},
		OnStop: func(ctx context.Context) error {
			return service.Stop(ctx)
		},
	})
	return service, service
}
