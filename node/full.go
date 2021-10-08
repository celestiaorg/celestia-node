package node

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
)

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *Config, repo Repository) fx.Option {
	return fx.Options(
		lightComponents(cfg, repo),
		fx.Provide(func() Type {
			return Full
		}),
		fxutil.ProvideIf(!cfg.Core.Remote, repo.Core), // provide core repo constructor only in embedded mode.
		nodecore.Components(cfg.Core),
		fx.Provide(func(client core.Client) block.Fetcher {
			return core.NewBlockFetcher(client)
		}),
		fx.Provide(block.NewBlockService),
	)
}
