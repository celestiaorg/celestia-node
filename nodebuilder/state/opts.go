package state

import (
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"go.uber.org/fx"
)

// WithKeyringSigner overrides the default keyring signer constructed
// by the node.
func WithKeyringSigner(signer *types.KeyringSigner) fx.Option {
	return fx.Replace(signer)
}
