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
	keyInfo, err := ring.Key(cfg.KeyringKeyName)
	if err != nil {
		log.Errorw("could not access key in keyring", "keyring.keyname", cfg.KeyringKeyName)
		return nil, "", err
	}
	return ring, AccountName(keyInfo.Name), nil
}
