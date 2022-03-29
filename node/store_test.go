package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoBridge(t *testing.T) {
	dir := t.TempDir()

	_, err := OpenStore(dir)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Bridge)
	require.NoError(t, err)

	store, err := OpenStore(dir)
	require.NoError(t, err)

	_, err = OpenStore(dir)
	assert.ErrorIs(t, err, ErrOpened)

	ks, err := store.Keystore()
	assert.NoError(t, err)
	assert.NotNil(t, ks)

	data, err := store.Datastore()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	cfg, err := store.Config()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestRepoLight(t *testing.T) {
	dir := t.TempDir()

	_, err := OpenStore(dir)
	assert.ErrorIs(t, err, ErrNotInited)

	err = Init(dir, Light)
	require.NoError(t, err)

	store, err := OpenStore(dir)
	require.NoError(t, err)

	_, err = OpenStore(dir)
	assert.ErrorIs(t, err, ErrOpened)

	ks, err := store.Keystore()
	assert.NoError(t, err)
	assert.NotNil(t, ks)

	data, err := store.Datastore()
	assert.NoError(t, err)
	assert.NotNil(t, data)

	cfg, err := store.Config()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}
