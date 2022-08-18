package state

import "github.com/celestiaorg/celestia-node/node/node"

// WithKeyringAccName sets the `KeyringAccName` field in the key config.
func WithKeyringAccName(name string) node.Option {
	return func(sets *node.Settings) {
		sets.Cfg.Key.KeyringAccName = name
	}
}
