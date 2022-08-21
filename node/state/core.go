package state

import (
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/service/state"
)

// CoreAccessor constructs a new instance of state.Accessor over
// a celestia-core connection.
func CoreAccessor(
	coreIP,
	coreRPC,
	coreGRPC string,
) func(*apptypes.KeyringSigner, header.Store) state.Accessor {
	return func(signer *apptypes.KeyringSigner, getter header.Store) state.Accessor {
		return state.NewCoreAccessor(signer, getter, coreIP, coreRPC, coreGRPC)
	}
}
