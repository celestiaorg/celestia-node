package state

import (
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/state"
)

// CoreAccessor constructs a new instance of state.Module over
// a celestia-core connection.
func CoreAccessor(
	corecfg core.Config,
	signer *apptypes.KeyringSigner,
	sync *sync.Syncer,
) *state.CoreAccessor {
	return state.NewCoreAccessor(signer, sync, corecfg.IP, corecfg.RPCPort, corecfg.GRPCPort)
}
