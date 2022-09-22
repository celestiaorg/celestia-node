package state

import (
	"go.uber.org/fx"

	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"

	"github.com/celestiaorg/celestia-node/header/sync"
	"github.com/celestiaorg/celestia-node/service/state"
)

// CoreAccessor constructs a new instance of state.Accessor over
// a celestia-core connection.
func CoreAccessor(
	coreIP,
	coreRPC,
	coreGRPC string,
) func(fx.Lifecycle, *apptypes.KeyringSigner, *sync.Syncer) (state.Accessor, error) {
	return func(lc fx.Lifecycle, signer *apptypes.KeyringSigner, syncer *sync.Syncer) (state.Accessor, error) {
		ca := state.NewCoreAccessor(signer, syncer, coreIP, coreRPC, coreGRPC)
		lc.Append(fx.Hook{
			OnStart: ca.Start,
			OnStop:  ca.Stop,
		})
		return ca, nil
	}
}
