package state

import "github.com/celestiaorg/celestia-node/node/config"

// WithKeyringAccName sets the `KeyringAccName` field in the key config.
func WithKeyringAccName(name string) config.Option {
	return func(sets *config.Settings) {
		sets.Cfg.Key.KeyringAccName = name
	}
}
