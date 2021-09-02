package node

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

// NewFull assembles a new Full Node from required components.
func NewFull(cfg *config.Config) (*Node, error) {
	return newNode(fullComponents(cfg))
}

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *config.Config) fx.Option {
	return fx.Options(
		// manual providing
		fx.Provide(func() Type {
			return Full
		}),
		fx.Provide(func() *config.Config {
			return cfg
		}),
		// components
		p2p.Components(cfg),
	)
}
