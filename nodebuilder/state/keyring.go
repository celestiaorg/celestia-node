package state

import (
	kr "github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-node/libs/keystore"
)

const DefaultAccountName = "my_celes_key"

type AccountName string

// Keyring constructs a new keyring.
// NOTE: we construct keyring before constructing node for easier UX
// as having keyring-backend set to `file` prompts user for password.
func Keyring(cfg Config, ks keystore.Keystore) (kr.Keyring, AccountName, error) {
	ring := ks.Keyring()
	var info *kr.Record
	// if custom keyringAccName provided, find key for that name
	if cfg.KeyringAccName != "" {
		keyInfo, err := ring.Key(cfg.KeyringAccName)
		if err != nil {
			log.Errorw("failed to find key by given name", "keyring.accname", cfg.KeyringAccName)
			return nil, "", err
		}
		info = keyInfo
	} else {
		// use default key
		keyInfo, err := ring.Key(DefaultAccountName)
		if err != nil {
			log.Errorw("could not access key in keyring", "name", DefaultAccountName)
			return nil, "", err
		}
		info = keyInfo
	}

	return ring, AccountName(info.Name), nil
}
