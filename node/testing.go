package node

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/service/block"
)

// MockRepository provides mock in memory Repository for testing purposes.
func MockRepository(t *testing.T, cfg *Config) Repository {
	t.Helper()
	repo := NewMemRepository()
	err := repo.PutConfig(cfg)
	require.NoError(t, err)
	crepo, err := repo.Core()
	require.NoError(t, err)
	err = crepo.PutConfig(core.MockConfig(t))
	require.NoError(t, err)
	return repo
}

// mockFullComponents keeps all components as DI options required to build a
// Full Node in **DEV MODE**:
// Dev mode constructs a mock embedded Core process that simulates block
// production for testing purposes.
func mockFullComponents(cfg *Config, repo Repository) fx.Option {
	return fx.Options(
		lightComponents(cfg, repo),
		fx.Provide(repo.Core),
		fx.Provide(core.MockEmbeddedClient),
		fx.Provide(func(client core.Client) block.Fetcher {
			return core.NewBlockFetcher(client)
		}),
		fx.Provide(block.NewBlockService),
	)
}
