package node

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/node/config"
	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
)

// NewFull assembles a new Full Node from required components.
func NewFull(cfg *config.Config, corecfg *core.Config) (*Node, error) {
	return newNode(fullComponents(cfg, corecfg))
}

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *config.Config, corecfg *core.Config) fx.Option {
	return fx.Options(
		lightComponents(cfg),
		fxutil.ProvideIf(!cfg.Core.Remote, func() *core.Config {
			return corecfg
		}),
		// components
		nodecore.Components(cfg.Core),
		fx.Provide(func(client core.Client) block.Fetcher {
			return core.NewBlockFetcher(client)
		}),
		fx.Provide(block.NewBlockService),
	)
}
