package state

import (
	"context"
	"errors"

	libfraud "github.com/celestiaorg/go-fraud"
	headersync "github.com/celestiaorg/go-header/sync"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/state"
)

var (
	log             = logging.Logger("module/state")
	ErrReadOnlyMode = errors.New("node is running in read-only mode")
)

// ConstructModule provides all components necessary to construct the
// state service.
func ConstructModule(tp node.Type, cfg *Config, coreCfg *core.Config) fx.Option {
	return ConstructModuleWithReadOnly(tp, cfg, coreCfg, false)
}

// ConstructModuleWithReadOnly provides all components necessary to construct the
// state service with optional read-only mode.
func ConstructModuleWithReadOnly(tp node.Type, cfg *Config, coreCfg *core.Config, readOnly bool) fx.Option {
	// sanitize config values before constructing module
	cfgErr := cfg.Validate()
	baseComponents := fx.Options(
		fx.Supply(*cfg),
		fx.Error(cfgErr),
		fx.Provide(func(ks keystore.Keystore) (keyring.Keyring, AccountName, error) {
			return Keyring(*cfg, ks)
		}),
		fxutil.ProvideIf(coreCfg.IsEndpointConfigured(), fx.Annotate(
			func(
				cfg Config,
				keyring keyring.Keyring,
				keyname AccountName,
				sync *headersync.Syncer[*header.ExtendedHeader],
				fraudServ libfraud.Service[*header.ExtendedHeader],
				network p2p.Network,
				client *grpc.ClientConn,
				additionalConns core.AdditionalCoreConns,
			) (
				*state.CoreAccessor,
				Module,
				*modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
				error,
			) {
				ca, mod, breaker, err := coreAccessor(cfg, keyring, keyname, sync, fraudServ, network, client, additionalConns)
				if readOnly {
					mod = &disabledStateModule{mod}
				}
				return ca, mod, breaker, err
			},
			fx.OnStart(func(ctx context.Context,
				breaker *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
			) error {
				return breaker.Start(ctx)
			}),
			fx.OnStop(func(ctx context.Context,
				breaker *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader],
			) error {
				return breaker.Stop(ctx)
			}),
		)),
		fxutil.ProvideIf(!coreCfg.IsEndpointConfigured(), func() (*state.CoreAccessor, Module) {
			var mod Module = &stubbedStateModule{}
			if readOnly {
				mod = &disabledStateModule{mod}
			}
			return nil, mod
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

// disabledStateModule is a wrapper that disables all write operations of the state module
type disabledStateModule struct {
	Module
}

func (s *disabledStateModule) Transfer(
	_ context.Context,
	_ state.AccAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) SubmitPayForBlob(
	_ context.Context,
	_ []*libshare.Blob,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) CancelUnbondingDelegation(
	_ context.Context,
	_ state.ValAddress,
	_, _ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) BeginRedelegate(
	_ context.Context,
	_, _ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) Undelegate(
	_ context.Context,
	_ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) Delegate(
	_ context.Context,
	_ state.ValAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) GrantFee(
	_ context.Context,
	_ state.AccAddress,
	_ state.Int,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}

func (s *disabledStateModule) RevokeGrantFee(
	_ context.Context,
	_ state.AccAddress,
	_ *state.TxConfig,
) (*state.TxResponse, error) {
	return nil, ErrReadOnlyMode
}
