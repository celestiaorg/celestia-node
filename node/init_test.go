package node

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/fslock"
)

// TODO(@Bidon15): We need more test coverage for Init part
// Tests could include invalid paths/configs and custom ones
// For more info visit #89
func TestInitFull(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir, Full)
	require.NoError(t, err)
	ok := IsInit(dir, Full)
	assert.True(t, ok)
}

func TestInitLight(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir, Light)
	require.NoError(t, err)
	ok := IsInit(dir, Light)
	assert.True(t, ok)
}

func TestInitErrForInvalidPath(t *testing.T) {
	path := "/invalid_path"
	errLight := Init(path, Light)
	errFull := Init(path, Full)

	require.Error(t, errLight)
	require.Error(t, errFull)
}

func TestInitErrForLockedDir(t *testing.T) {
	dir := t.TempDir()
	flock, err := fslock.Lock(lockPath(dir))
	require.NoError(t, err)
	defer flock.Unlock() //nolint:errcheck

	errLight := Init(dir, Light)
	errFull := Init(dir, Full)

	require.Error(t, errLight)
	require.Error(t, errFull)
}

func TestIsInitWithBrokenConfig(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(configPath(dir))
	require.NoError(t, err)
	defer f.Close()

	//nolint:errcheck
	f.Write([]byte(`
		[P2P]
		  ListenAddresses = [/ip4/0.0.0.0/tcp/2121]
    `))

	okLight := IsInit(dir, Light)
	okFull := IsInit(dir, Full)

	assert.False(t, okLight)
	assert.False(t, okFull)
}

func TestIsInitForNonExistDir(t *testing.T) {
	okLight := IsInit("/invalid_path", Light)
	okFull := IsInit("/invalid_path", Full)

	assert.False(t, okLight)
	assert.False(t, okFull)
}

func TestInitWithNilCfg(t *testing.T) {
	dir := t.TempDir()
	err := InitWith(dir, Light, nil)

	require.Error(t, err)
}
