//go:build test_unit

package node

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/fslock"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	nodes := []Type{Light, Bridge}

	for _, node := range nodes {
		require.NoError(t, Init(dir, node))
		assert.True(t, IsInit(dir))
	}
}

func TestInitErrForInvalidPath(t *testing.T) {
	path := "/invalid_path"
	nodes := []Type{Light, Bridge}

	for _, node := range nodes {
		require.Error(t, Init(path, node))
	}
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
	assert.False(t, IsInit(dir))
}

func TestIsInitForNonExistDir(t *testing.T) {
	path := "/invalid_path"
	assert.False(t, IsInit(path))
}

func TestInitErrForLockedDir(t *testing.T) {
	dir := t.TempDir()
	flock, err := fslock.Lock(lockPath(dir))
	require.NoError(t, err)
	defer flock.Unlock() //nolint:errcheck
	nodes := []Type{Light, Bridge}

	for _, node := range nodes {
		require.Error(t, Init(dir, node))
	}
}
