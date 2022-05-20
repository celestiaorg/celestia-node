package state

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apptypes "github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/params"
	"github.com/celestiaorg/celestia-node/service/state"
)

var log = logging.Logger("state-access-constructor")

func CoreAccessor(
	endpoint,
	keyringAccName string,
) func(fx.Lifecycle, keystore.Keystore, params.Network) (state.Accessor, error) {
	return func(lc fx.Lifecycle, ks keystore.Keystore, net params.Network) (state.Accessor, error) {
		// TODO @renaynay: Include option for setting custom `userInput` parameter with
		//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
		// TODO @renaynay @Wondertan: ensure that keyring backend from config is passed
		//  here instead of hardcoded `BackendTest`: https://github.com/celestiaorg/celestia-node/issues/603.
		encConf := encoding.MakeEncodingConfig(app.ModuleBasics.RegisterInterfaces)
		ring, err := keyring.New(app.Name, keyring.BackendTest, ks.Path(), os.Stdin, encConf.Codec)
		if err != nil {
			return nil, err
		}
		signer := apptypes.NewKeyringSigner(ring, keyringAccName, string(net))

		var info *keyring.Record
		// if custom keyringAccName provided, find key for that name
		if keyringAccName != "" {
			keyInfo, err := signer.Key(keyringAccName)
			if err != nil {
				return nil, err
			}
			info = keyInfo
		} else {
			// check if key exists for signer
			keys, err := signer.List()
			if err != nil {
				return nil, err
			}
			// if no key was found in keystore path, generate new key for node
			if len(keys) == 0 {
				log.Infow("NO KEY FOUND IN STORE, GENERATING NEW KEY...", "path", ks.Path())
				keyInfo, mn, err := signer.NewMnemonic("my_celes_key", keyring.English, "",
					"", hd.Secp256k1)
				if err != nil {
					return nil, err
				}
				log.Info("NEW KEY GENERATED...")
				addr, err := keyInfo.GetAddress()
				if err != nil {
					return nil, err
				}
				fmt.Printf("\nNAME: %s\nADDRESS: %s\nMNEMONIC (save this somewhere safe!!!): \n%s\n\n",
					keyInfo.Name, addr.String(), mn)

				info = keyInfo
			} else {
				// if one or more keys are present and no keyringAccName was given, use the first key in list
				info = keys[0]
			}
		}

		log.Infow("constructed keyring signer", "backend", keyring.BackendTest, "path", ks.Path(),
			"key name", info.Name, "chain-id", string(net))

		ca := state.NewCoreAccessor(signer, endpoint)
		lc.Append(fx.Hook{
			OnStart: ca.Start,
			OnStop:  ca.Stop,
		})
		return ca, nil
	}
}
