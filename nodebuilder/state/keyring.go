package state

import (
	"fmt"

	kr "github.com/cosmos/cosmos-sdk/crypto/keyring"

	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

// KeyringSigner constructs a new keyring signer.
// NOTE: we construct keyring signer before constructing node for easier UX
// as having keyring-backend set to `file` prompts user for password.
func KeyringSigner(cfg Config, ks keystore.Keystore, net p2p.Network) (*apptypes.KeyringSigner, error) {
	ring := ks.Keyring()
	var info *kr.Record
	// if custom keyringAccName provided, find key for that name
	if cfg.KeyringAccName != "" {
		keyInfo, err := ring.Key(cfg.KeyringAccName)
		if err != nil {
			log.Errorw("failed to find key by given name", "keyring.accname", cfg.KeyringAccName)
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
			log.Errorw("no keys found in path", "path", ks.Path(), "keyring backend",
				cfg.KeyringBackend)
			return nil, fmt.Errorf("no keys found in path %s using keyring backend %s", ks.Path(),
				cfg.KeyringBackend)
		}
		// if one or more keys are present and no keyringAccName was given, use the first key in list
		keyInfo, err := ring.Key(keys[0].Name)
		if err != nil {
			log.Errorw("could not access key in keyring", "name", keys[0].Name)
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
