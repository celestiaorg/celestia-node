package state

import "github.com/cosmos/cosmos-sdk/crypto/keyring"

var defaultKeyringBackend = keyring.BackendTest

// Config contains configuration parameters for constructing
// the node's keyring signer.
type Config struct {
	KeyringAccName string
	KeyringBackend string
}

func DefaultConfig() Config {
	return Config{
		KeyringAccName: "",
		KeyringBackend: defaultKeyringBackend,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	return nil
}
