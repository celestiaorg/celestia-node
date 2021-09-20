package node

import (
	"context"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/node/p2p"
)

// NewLight assembles a new Light Node from required components.
func NewLight(cfg *config.Config) (*Node, error) {
	return newNode(lightComponents(cfg))
}

// lightComponents keeps all the components as DI options required to built a Light Node.
func lightComponents(cfg *config.Config) fx.Option {
	return fx.Options(
		// manual providing
		fx.Provide(context.Background),
		fx.Provide(func() Type {
			return Light
		}),
		fx.Provide(func() *config.Config {
			return cfg
		}),
		// components
		p2p.Components(cfg.P2P),
	)
}
