package state

import (
	kr "github.com/cosmos/cosmos-sdk/crypto/keyring"

	apptypes "github.com/celestiaorg/celestia-app/x/blob/types"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

const DefaultAccountName = "my_celes_key"

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
		// use default key
		keyInfo, err := ring.Key(DefaultAccountName)
		if err != nil {
			log.Errorw("could not access key in keyring", "name", DefaultAccountName)
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
