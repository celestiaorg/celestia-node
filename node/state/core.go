package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/app"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/state"
)

var keyringAccName = "default"

func CoreAccessor(endpoint string) func(fx.Lifecycle, keystore.Keystore, params.Network) (state.Accessor, error) {
	return func(lc fx.Lifecycle, ks keystore.Keystore, net params.Network) (state.Accessor, error) {
		// TODO @renaynay: Include option for setting custom `userInput` parameter with
		//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
		ring, err := keyring.New(app.Name, keyring.BackendFile, ks.Path(), nil)
		if err != nil {
			return nil, err
		}

		ca := state.NewCoreAccessor(apptypes.NewKeyringSigner(ring, keyringAccName, string(net)), endpoint)
		lc.Append(fx.Hook{
			OnStart: ca.Start,
			OnStop:  ca.Stop,
		})
		return ca, nil
	}
}
