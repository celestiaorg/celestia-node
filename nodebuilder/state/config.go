package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

var defaultBackendName = keyring.BackendTest

// Config contains configuration parameters for constructing
// the node's keyring signer.
type Config struct {
	DefaultKeyName     string
	DefaultBackendName string
}

func DefaultConfig() Config {
	return Config{
		DefaultKeyName:     DefaultKeyName,
		DefaultBackendName: defaultBackendName,
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	return nil
}
