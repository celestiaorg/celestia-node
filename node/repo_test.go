package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(dir)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, DefaultConfig())
	require.NoError(t, err)

	repo, err := Open(dir)
	require.NoError(t, err)

	_, err = Open(dir)
	assert.ErrorIs(t, err, ErrOpened)

	ks, err := repo.Keystore()
	assert.NoError(t, err)
	assert.NotNil(t, ks)

	data, err := repo.Datastore()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	crepo, err := repo.Core()
	assert.NoError(t, err)
	assert.NotNil(t, crepo)
}
