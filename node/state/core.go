package state

import (
	"go.uber.org/fx"

	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/service/state"
)

// CoreAccessor constructs a new instance of state.Accessor over
// a celestia-core connection.
func CoreAccessor(
	endpoint string,
) func(fx.Lifecycle, *apptypes.KeyringSigner) (state.Accessor, error) {
	return func(lc fx.Lifecycle, signer *apptypes.KeyringSigner) (state.Accessor, error) {
		ca := state.NewCoreAccessor(signer, endpoint)
		lc.Append(fx.Hook{
			OnStart: ca.Start,
			OnStop:  ca.Stop,
		})
		return ca, nil
	}
}
