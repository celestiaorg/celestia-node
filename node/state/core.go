package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-app/app"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/state"
)

var keyringAccName = "default"

func CoreAccessor(
	ks keystore.Keystore,
	coreEndpoint string,
) (state.Accessor, error) {
	// TODO @renaynay: Include option for setting custom `userInput` parameter with
	//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
	ring, err := keyring.New(app.Name, keyring.BackendFile, ks.Path(), nil)
	if err != nil {
		return nil, err
	}
	signer := apptypes.NewKeyringSigner(ring, keyringAccName, string(params.GetNetwork()))

	return state.NewCoreAccessor(signer, coreEndpoint), nil
}
