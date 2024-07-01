package blob

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	headerService "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/share"
)

func ConstructModule() fx.Option {
	return fx.Module("blob",
		fx.Provide(
			func(service headerService.Module) func(context.Context, uint64) (*header.ExtendedHeader, error) {
				return service.GetByHeight
			},
		),
		fx.Provide(
			func(service headerService.Module) func(context.Context) (<-chan *header.ExtendedHeader, error) {
				return service.Subscribe
			},
		),
		fx.Provide(fx.Annotate(
			func(
				state state.Module,
				sGetter share.Getter,
				getByHeightFn func(context.Context, uint64) (*header.ExtendedHeader, error),
				subscribeFn func(context.Context) (<-chan *header.ExtendedHeader, error),
			) *blob.Service {
				return blob.NewService(state, sGetter, getByHeightFn, subscribeFn)
			}),
			fx.OnStart(func(ctx context.Context, serv *blob.Service) error {
				return serv.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, serv *blob.Service) error {
				return serv.Stop(ctx)
			}),
		),
		fx.Provide(func(serv *blob.Service) Module {
			return serv
		}),
	)
}
