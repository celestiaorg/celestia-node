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
		fx.Provide(func(
			state state.Module,
			sGetter share.Getter,
			getByHeightFn func(context.Context, uint64) (*header.ExtendedHeader, error),
			subscribeFn func(context.Context) (<-chan *header.ExtendedHeader, error),
		) Module {
			return blob.NewService(state, sGetter, getByHeightFn, subscribeFn)
		}),
	)
}
