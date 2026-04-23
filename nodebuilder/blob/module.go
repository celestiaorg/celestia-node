package blob

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	headerService "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func ConstructModule() fx.Option {
	return fx.Module("blob",
		// GetByHeight is provided at module scope because other modules
		// (e.g. nodebuilder/da) depend on this plain func signature.
		fx.Provide(
			func(service headerService.Module) func(context.Context, uint64) (*header.ExtendedHeader, error) {
				return service.GetByHeight
			},
		),
		// WaitForHeight and Subscribe are consumed directly inside the
		// Service constructor below rather than as separate fx providers,
		// so fx doesn't have to disambiguate two providers that share the
		// GetByHeight func signature.
		fx.Provide(fx.Annotate(
			func(
				state state.Module,
				sGetter shwap.Getter,
				getByHeight func(context.Context, uint64) (*header.ExtendedHeader, error),
				headerMod headerService.Module,
			) *blob.Service {
				return blob.NewService(
					state,
					sGetter,
					getByHeight,
					headerMod.WaitForHeight,
					headerMod.Subscribe,
				)
			},
			fx.OnStart(func(ctx context.Context, serv *blob.Service) error {
				return serv.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, serv *blob.Service) error {
				return serv.Stop(ctx)
			}),
		)),
		fx.Provide(func(serv *blob.Service) Module {
			return serv
		}),
	)
}
