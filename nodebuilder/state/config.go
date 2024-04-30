package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

var defaultKeyringBackend = keyring.BackendTest

// Config contains configuration parameters for constructing
// the node's keyring signer.
type Config struct {
	KeyringKeyNames []string
	KeyringBackend  string
}

func DefaultConfig() Config {
	return Config{
		KeyringKeyNames: []string{},
		KeyringBackend:  defaultKeyringBackend,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	return nil
}
