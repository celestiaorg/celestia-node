package state

import (
	"context"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
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
		fx.Error(cfgErr),
		fx.Provide(func(ks keystore.Keystore) (keyring.Keyring, AccountName, error) {
			return Keyring(*cfg, ks)
		}),
		fxutil.ProvideIf(coreCfg.IsEndpointConfigured(), fx.Annotate(
			coreAccessor,
			fx.OnStart(func(ctx context.Context, state *state.CoreAccessor) error {
				return state.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context, state *state.CoreAccessor) error {
				return state.Stop(ctx)
			}),
		)),
		fxutil.ProvideIf(!coreCfg.IsEndpointConfigured(), func() (*state.CoreAccessor, Module) {
			return nil, &stubbedStateModule{}
		}),
	)

	switch tp {
	case node.Light, node.Full, node.Bridge, node.Pin:
		return fx.Module(
			"state",
			baseComponents,
		)
	default:
		panic("invalid node type")
	}
}
