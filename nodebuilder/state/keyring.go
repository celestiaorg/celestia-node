package state

import (
	"fmt"
	"os"

	kr "github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func keyring(cfg Config, ks keystore.Keystore, net p2p.Network) (*apptypes.KeyringSigner, error) {
	// TODO @renaynay: Include option for setting custom `userInput` parameter with
	//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	ring, err := kr.New(app.Name, cfg.KeyringBackend, ks.Path(), os.Stdin, encConf.Codec)
	if err != nil {
		return nil, err
	}

	var info *kr.Record
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
			return nil, fmt.Errorf("no keys found in path %s using keyring backend %s", ks.Path(),
				cfg.KeyringBackend)
		}
		// if one or more keys are present and no keyringAccName was given, use the first key in list
		keyInfo, err := ring.Key(keys[0].Name)
		if err != nil {
			return nil, err
		}
		info = keyInfo
	}
	// construct signer using the default key found / generated above
	signer := apptypes.NewKeyringSigner(ring, info.Name, string(net))
	signerInfo := signer.GetSignerInfo()
	log.Infow("constructed keyring signer", "backend", cfg.KeyringBackend, "path", ks.Path(),
		"key name", signerInfo.Name, "chain-id", string(net))

	return signer, nil
}
