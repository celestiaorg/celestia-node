package state

import (
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/state"

	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
)

// CoreAccessor constructs a new instance of state.Service over
// a celestia-core connection.
func CoreAccessor(corecfg core.Config, signer *apptypes.KeyringSigner, sync *sync.Syncer) Service {
	return state.NewCoreAccessor(signer, sync, corecfg.IP, corecfg.RPCPort, corecfg.GRPCPort)
}
