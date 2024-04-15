package state

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	libfraud "github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/state"
)

var log = logging.Logger("module/state")

// ConstructModule provides all components necessary to construct the
// state service.
func ConstructModule(tp node.Type, cfg *Config, coreCfg *core.Config) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()

	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Supply(cfg.GranterAddress),
		fx.Error(cfgErr),
		fxutil.ProvideIf(coreCfg.IsEndpointConfigured(), fx.Annotate(
			func(
				signer *apptypes.KeyringSigner,
				sync *sync.Syncer[*header.ExtendedHeader],
				fraudServ libfraud.Service[*header.ExtendedHeader],
			) (*state.CoreAccessor, Module, *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]) {
				return coreAccessor(*coreCfg, signer, sync, fraudServ, state.WithGranter(cfg.GranterAddress))
			},
			fx.OnStart(func(ctx context.Context,
				breaker *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context,
				breaker *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]) error {
				return breaker.Stop(ctx)
			}),
		)),
		fxutil.ProvideIf(!coreCfg.IsEndpointConfigured(), func() (*state.CoreAccessor, Module) {
			return nil, &stubbedStateModule{}
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
