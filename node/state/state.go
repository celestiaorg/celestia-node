package state

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/node/key"
	"github.com/celestiaorg/celestia-node/node/services"
	"github.com/celestiaorg/celestia-node/service/state"
)

var log = logging.Logger("state-access-constructor")

// Components provides all components necessary to construct the
// state service.
func Components(coreEndpoint string, cfg key.Config) fx.Option {
	return fx.Options(
		fx.Provide(Keyring(cfg)),
		fx.Provide(CoreAccessor(coreEndpoint)),
		fx.Provide(Service),
	)
}

// Service constructs a new state.Service.
func Service(ctx context.Context, lc fx.Lifecycle, accessor state.Accessor, fservice fraud.Service) *state.Service {
	serv := state.NewService(accessor)
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return services.FraudedLifecycle(fxutil.WithLifecycle(ctx, lc), fraud.BadEncoding, fservice, serv.Start, serv.Stop)
		},
		OnStop: serv.Stop,
	})

	return serv
}
