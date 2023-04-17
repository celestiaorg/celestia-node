package state

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/state"
)

var log = logging.Logger("module/state")

// ConstructModule provides all components necessary to construct the
// state service.
func ConstructModule(tp node.Type, cfg *Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(fx.Annotate(
			coreAccessor,
			fx.OnStart(func(ctx context.Context, breaker *modfraud.ServiceBreaker[*state.CoreAccessor]) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, breaker *modfraud.ServiceBreaker[*state.CoreAccessor]) error {
				return breaker.Stop(ctx)
			}),
		)),
		// the module is needed for the handler
		fx.Provide(func(ca *state.CoreAccessor) Module {
			return ca
		}),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge:
		return fx.Module(
			"state",
			baseComponents,
		)
	default:
		panic("invalid node type")
	}
}
