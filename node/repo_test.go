package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoFull(t *testing.T) {
	dir := t.TempDir()

	_, err := Open(dir, Full)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Full)
	require.NoError(t, err)

	repo, err := Open(dir, Full)
	require.NoError(t, err)

	_, err = Open(dir, Full)
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
}
