package node

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

// node.NewLight assembles a new Light Node from required components.
func NewLight(cfg *config.Config) (*Node, error) {
	return newNode(lightComponents(cfg))
}

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *config.Config) fx.Option {
	return fx.Options(
		fx.Provide(func() Type {
			return Light
		}),
		fx.Provide(func() *config.Config {
			return cfg
		}),

		p2p.P2P(cfg),
	)
}
