package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoBridge(t *testing.T) {
	dir := t.TempDir()

	_, err := OpenStore(dir, Bridge)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Bridge)
	require.NoError(t, err)

	store, err := OpenStore(dir, Bridge)
	require.NoError(t, err)

	_, err = OpenStore(dir, Bridge)
	assert.ErrorIs(t, err, ErrOpened)

	ks, err := store.Keystore()
	assert.NoError(t, err)
	assert.NotNil(t, ks)

	data, err := store.Datastore()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	cstore, err := store.Core()
	assert.NoError(t, err)
	assert.NotNil(t, cstore)

	cfg, err := store.Config()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestRepoLight(t *testing.T) {
	dir := t.TempDir()

	_, err := OpenStore(dir, Light)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Light)
	require.NoError(t, err)

	store, err := OpenStore(dir, Light)
	require.NoError(t, err)

	_, err = OpenStore(dir, Light)
	assert.ErrorIs(t, err, ErrOpened)

	ks, err := store.Keystore()
	assert.NoError(t, err)
	assert.NotNil(t, ks)

	data, err := store.Datastore()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	cstore, err := store.Core()
	assert.Error(t, err)
	assert.Nil(t, cstore)

	cfg, err := store.Config()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}
