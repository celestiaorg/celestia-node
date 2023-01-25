package state

import (
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func Keyring(cfg Config, ks keystore.Keystore, net p2p.Network) (*apptypes.KeyringSigner, error) {
	// TODO @renaynay: Include option for setting custom `userInput` parameter with
	//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
	// TODO @renaynay @Wondertan: ensure that keyring backend from config is passed
	//  here instead of hardcoded `BackendTest`:
	// https://github.com/celestiaorg/celestia-node/issues/603.
	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	ring, err := keyring.New(app.Name, keyring.BackendTest, ks.Path(), os.Stdin, encConf.Codec)
	if err != nil {
		return nil, err
	}

	var info *keyring.Record
	// if custom keyringAccName provided, find key for that name
	if cfg.KeyringAccName != "" {
		keyInfo, err := ring.Key(cfg.KeyringAccName)
		if err != nil {
			return nil, err
		}
		info = keyInfo
	} else {
		// check if key exists for signer
		keys, err := ring.List()
		if err != nil {
			return nil, err
		}
		// if no key was found in keystore path, generate new key for node
		if len(keys) == 0 {
			log.Infow("NO KEY FOUND IN STORE, GENERATING NEW KEY...", "path", ks.Path())
			keyInfo, mn, err := ring.NewMnemonic("my_celes_key", keyring.English, "",
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
	// construct signer using the default key found / generated above
	signer := apptypes.NewKeyringSigner(ring, info.Name, string(net))
	signerInfo := signer.GetSignerInfo()
	log.Infow("constructed keyring signer", "backend", keyring.BackendTest, "path", ks.Path(),
		"key name", signerInfo.Name, "chain-id", string(net))

	return signer, nil
}
