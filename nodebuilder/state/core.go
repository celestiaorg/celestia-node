package state

import (
	"context"

	"go.uber.org/fx"

	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	libfraud "github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	modfraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/state"
)

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection with enabled granter.
func coreAccessorWithGranter(
	lc fx.Lifecycle,
	corecfg core.Config,
	signer *apptypes.KeyringSigner,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service[*header.ExtendedHeader],
) (*state.CoreAccessor, Module, *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]) {
	return coreAccessor(lc, corecfg, signer, sync, fraudServ, true)
}

func coreAccessorWithoutGranter(
	lc fx.Lifecycle,
	corecfg core.Config,
	signer *apptypes.KeyringSigner,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service[*header.ExtendedHeader],
) (*state.CoreAccessor, Module, *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]) {
	return coreAccessor(lc, corecfg, signer, sync, fraudServ, false)
}

// coreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func coreAccessor(
	lc fx.Lifecycle,
	corecfg core.Config,
	signer *apptypes.KeyringSigner,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service[*header.ExtendedHeader],
	granterEnabled bool,
) (*state.CoreAccessor, Module, *modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]) {
	ca := state.NewCoreAccessor(signer, sync, corecfg.IP, corecfg.RPCPort, corecfg.GRPCPort, granterEnabled)
	sBreaker := &modfraud.ServiceBreaker[*state.CoreAccessor, *header.ExtendedHeader]{
		Service:   ca,
		FraudType: byzantine.BadEncoding,
		FraudServ: fraudServ,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return sBreaker.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return sBreaker.Stop(ctx)
		},
	})
	return ca, ca, sBreaker
}
