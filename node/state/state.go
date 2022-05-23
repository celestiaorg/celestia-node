package state

import (
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/key"
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
func Service(accessor state.Accessor) *state.Service {
	return state.NewService(accessor)
}
