package node

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	nodecore "github.com/celestiaorg/celestia-node/node/core"
	"github.com/celestiaorg/celestia-node/node/fxutil"
	"github.com/celestiaorg/celestia-node/service/block"
)

// NewFull assembles a new Full Node from required components.
func NewFull(repo Repository) (*Node, error) {
	cfg, err := repo.Config()
	if err != nil {
		return nil, err
	}

	return newNode(fullComponents(cfg, repo))
}

// fullComponents keeps all the components as DI options required to built a Full Node.
func fullComponents(cfg *Config, repo Repository) fx.Option {
	return fx.Options(
		lightComponents(cfg, repo),
		fxutil.ProvideIf(!cfg.Core.Remote, repo.Core), // provide core repo constructor only in embedded mode.
		nodecore.Components(cfg.Core),
		fx.Provide(func(client core.Client) block.Fetcher {
			return core.NewBlockFetcher(client)
		}),
		fx.Provide(block.NewBlockService),
	)
}
