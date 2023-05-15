package blob

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	headerService "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/state"
)

func ConstructModule() fx.Option {
	return fx.Module("blob",
		fxutil.ProvideAs(
			func(service headerService.Module) func(context.Context, uint64) (*header.ExtendedHeader, error) {
				return service.GetByHeight
			}),
		fx.Provide(fx.Annotate(func(
			state *state.CoreAccessor,
			sGetter share.Getter,
			getByHeightFn func(context.Context, uint64) (*header.ExtendedHeader, error),
		) Module {
			return blob.NewService(state, sGetter, getByHeightFn)
		})))
}
