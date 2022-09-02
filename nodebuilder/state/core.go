package state

import (
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/service/state"
)

// CoreAccessor constructs a new instance of state.Accessor over
// a celestia-core connection.
func CoreAccessor(corecfg core.Config, signer *apptypes.KeyringSigner, getter header.Store) state.Accessor {
	return state.NewCoreAccessor(signer, getter, corecfg.IP, corecfg.RPCPort, corecfg.RPCPort)
}
