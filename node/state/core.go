package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-app/app"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/node/services"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/state"
)

func CoreAccessor(
	keyCfg services.KeyConfig,
	storePath string,
	coreEndpoint string,
) (state.Accessor, error) {
	ring, err := keyring.New(app.Name, keyring.BackendFile, storePath, nil) // TODO @renaynay: user input?
	if err != nil {
		return nil, err
	}
	signer := apptypes.NewKeyringSigner(ring, keyCfg.KeyringAccName, string(params.GetNetwork()))

	return state.NewCoreAccessor(signer, coreEndpoint), nil
}
