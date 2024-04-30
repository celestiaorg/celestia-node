package state

import (
	"fmt"

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

	// go through all keys in the config and check their availability in the KeyStore.
	for _, accName := range cfg.KeyringKeyNames {
		keyInfo, err := ring.Key(accName)
		if err != nil {
			err = fmt.Errorf("key not found in keystore: %s", accName)
			return nil, "", err
		}
		if info == nil {
			info = keyInfo
		}
	}
	// set the default key in case config does not provide any keys.
	if info == nil {
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
