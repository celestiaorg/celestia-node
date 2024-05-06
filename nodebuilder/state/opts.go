package state

import (
	kr "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/libs/fxutil"
)

// WithKeyring overrides the default keyring constructed
// by the node.
func WithKeyring(keyring kr.Keyring) fx.Option {
	return fxutil.ReplaceAs(keyring, new(kr.Keyring))
}

// WithKeyName configures the signer to use the given key.
func WithKeyName(name AccountName) fx.Option {
	return fx.Replace(name)
}
