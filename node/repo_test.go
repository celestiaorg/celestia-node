package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:dupl
func TestRepoBridge(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(dir, Bridge)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Bridge)
	require.NoError(t, err)

	repo, err := Open(dir, Bridge)
	require.NoError(t, err)

	_, err = Open(dir, Bridge)
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

	cfg, err := repo.Config()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

//nolint:dupl
func TestRepoLight(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(dir, Light)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Light)
	require.NoError(t, err)

	repo, err := Open(dir, Light)
	require.NoError(t, err)

	_, err = Open(dir, Light)
	assert.ErrorIs(t, err, ErrOpened)

	ks, err := repo.Keystore()
	assert.NoError(t, err)
	assert.NotNil(t, ks)

	data, err := repo.Datastore()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	crepo, err := repo.Core()
	assert.Error(t, err)
	assert.Nil(t, crepo)

	cfg, err := repo.Config()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}
