package node

import (
	"context"

	"go.uber.org/fx"

	coreclient "github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/celestiaorg/celestia-node/service/block"
)

// NewFull assembles a new Full Node from required components.
func NewFull(cfg *Config) (*Node, error) {
	return newNode(fullComponents(cfg))
}

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *Config) fx.Option {
	return fx.Options(
		// manual providing
		fx.Provide(context.Background),
		fx.Provide(func() Type {
			return Full
		}),
		fx.Provide(func() *Config {
			return cfg
		}),
		// provide the block Fetcher
		fx.Provide(func(client coreclient.Client) block.Fetcher {
			return coreclient.NewBlockFetcher(client)
		}),
		fx.Provide(block.NewBlockService),
		// components
		p2p.Components(cfg.P2P),
		core.Components(cfg.Core),
		fx.Provide(block.NewBlockService),
	)
}
