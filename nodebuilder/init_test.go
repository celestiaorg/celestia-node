package nodebuilder

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	nodes := []node.Type{node.Light, node.Bridge}

	for _, node := range nodes {
		cfg := DefaultConfig(node)
		require.NoError(t, Init(*cfg, dir, node))
		assert.True(t, IsInit(dir))
	}
}

func TestInitErrForInvalidPath(t *testing.T) {
	path := "/invalid_path"
	nodes := []node.Type{node.Light, node.Bridge}

	for _, node := range nodes {
		cfg := DefaultConfig(node)
		require.Error(t, Init(*cfg, path, node))
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
	nodes := []node.Type{node.Light, node.Bridge}

	for _, node := range nodes {
		cfg := DefaultConfig(node)
		require.Error(t, Init(*cfg, dir, node))
	}
}
