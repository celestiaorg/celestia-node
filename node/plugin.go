package node

import "github.com/celestiaorg/celestia-node/node/fxutil"

type Plugin interface {
	Name() string
	Initialize(path string) error
	Components(cfg *Config, store Store) fxutil.Option
}
