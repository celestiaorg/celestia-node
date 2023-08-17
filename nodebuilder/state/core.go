package state

import (
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
// a celestia-core connection.
func coreAccessor(
	corecfg core.Config,
	signer *apptypes.KeyringSigner,
	sync *sync.Syncer[*header.ExtendedHeader],
	fraudServ libfraud.Service,
) (*state.CoreAccessor, *modfraud.ServiceBreaker[*state.CoreAccessor]) {
	if !corecfg.EndpointConfigured() {
		log.Info("No core endpoint provided, running node without state access")
		return nil, nil
	}

	ca := state.NewCoreAccessor(signer, sync, corecfg.IP, corecfg.RPCPort, corecfg.GRPCPort)

	return ca, &modfraud.ServiceBreaker[*state.CoreAccessor]{
		Service:   ca,
		FraudType: byzantine.BadEncoding,
		FraudServ: fraudServ,
	}
}
