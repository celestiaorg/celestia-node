package blob

import (
	"context"
	"errors"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	headerService "github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

var ErrReadOnlyMode = errors.New("node is running in read-only mode")

func ConstructModule() fx.Option {
	return ConstructModuleWithReadOnly(false)
}

func ConstructModuleWithReadOnly(readOnly bool) fx.Option {
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
				sGetter shwap.Getter,
				getByHeightFn func(context.Context, uint64) (*header.ExtendedHeader, error),
				subscribeFn func(context.Context) (<-chan *header.ExtendedHeader, error),
			) *blob.Service {
				return blob.NewService(state, sGetter, getByHeightFn, subscribeFn)
			},
			fx.OnStart(func(ctx context.Context, serv *blob.Service) error {
				return serv.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, serv *blob.Service) error {
				return serv.Stop(ctx)
			}),
		)),
		fx.Provide(func(serv *blob.Service) Module {
			var mod Module = serv
			if readOnly {
				mod = &readOnlyBlobModule{mod}
			}
			return mod
		}),
	)
}

// readOnlyBlobModule is a wrapper that disables the Submit operation of the blob module
type readOnlyBlobModule struct {
	Module
}

func (b *readOnlyBlobModule) Submit(
	_ context.Context,
	_ []*blob.Blob,
	_ *blob.SubmitOptions,
) (uint64, error) {
	return 0, ErrReadOnlyMode
}
