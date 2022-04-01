package state

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/app"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/state"
)

var keyringAccName = "celes"

func CoreAccessor(endpoint string, tp string) func(fx.Lifecycle, params.Network) (state.Accessor, error) {
	return func(lc fx.Lifecycle, net params.Network) (state.Accessor, error) {
		keyPath := fmt.Sprintf("~/.celestia-%s/keys", tp)

		// TODO @renaynay: Include option for setting custom `userInput` parameter with
		//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
		ring, err := keyring.New(app.Name, keyring.BackendTest, keyPath, os.Stdin)
		if err != nil {
			return nil, err
		}
		signer := apptypes.NewKeyringSigner(ring, keyringAccName, string(net))

		ca, err := state.NewCoreAccessor(signer, endpoint), nil
		if err != nil {
			return nil, err
		}
		lc.Append(fx.Hook{
			OnStart: ca.Start,
			OnStop:  ca.Stop,
		})
		return ca, nil
	}
}
