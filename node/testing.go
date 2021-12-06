package node

import (
	"testing"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/stretchr/testify/require"
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
