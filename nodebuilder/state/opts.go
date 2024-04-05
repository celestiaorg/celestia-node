package state

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/fx"
)

// WithKeyringSigner overrides the default keyring signer constructed
// by the node.
func WithKeyring(keyring keyring.Keyring) fx.Option {
	return fx.Replace(keyring)
}

func WithKeyName(name string) fx.Option {
	return fx.Replace(name)
}
