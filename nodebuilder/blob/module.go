package blob

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/fibre"
	"github.com/celestiaorg/celestia-node/header"
	headerService "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/share/shwap"
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
		fx.Provide(func(
			state state.Module,
			sGetter shwap.Getter,
			getByHeightFn func(context.Context, uint64) (*header.ExtendedHeader, error),
			subscribeFn func(context.Context) (<-chan *header.ExtendedHeader, error),
			params struct {
				fx.In
				FibreClient *fibre.Client `optional:"true"`
			},
		) *blob.Service {
			var fibreSubmitter blob.FibreSubmitter
			if params.FibreClient != nil {
				fibreSubmitter = params.FibreClient
			}
			return blob.NewService(state, fibreSubmitter, sGetter, getByHeightFn, subscribeFn)
		}),
		fx.Invoke(func(lc fx.Lifecycle, serv *blob.Service) {
			lc.Append(fx.Hook{
				OnStart: serv.Start,
				OnStop:  serv.Stop,
			})
		}),
		fx.Provide(func(serv *blob.Service) Module {
			return serv
		}),
	)
}
