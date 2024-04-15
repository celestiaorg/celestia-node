package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/celestiaorg/celestia-node/state"
)

var defaultKeyringBackend = keyring.BackendTest

// Config contains configuration parameters for constructing
// the node's keyring signer.
type Config struct {
	KeyringAccName string
	KeyringBackend string
	GranterAddress state.AccAddress
}

func DefaultConfig() Config {
	return Config{
		KeyringAccName: "",
		KeyringBackend: defaultKeyringBackend,
		GranterAddress: state.AccAddress{},
	}
}

// Validate performs basic validation of the config.
func (cfg *Config) Validate() error {
	return nil
}
