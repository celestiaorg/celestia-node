package node

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
)

func MockRepository(t *testing.T) Repository {
	t.Helper()
	repo := NewMemRepository()
	err := repo.PutConfig(DefaultConfig())
	require.NoError(t, err)
	crepo, err := repo.Core()
	require.NoError(t, err)
	err = crepo.PutConfig(core.MockConfig(t))
	require.NoError(t, err)
	return repo
}
