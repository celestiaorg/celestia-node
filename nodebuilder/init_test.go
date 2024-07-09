package nodebuilder

import (
	"os"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/app/encoding"

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
	flk := flock.New(lockPath(dir))
	_, err := flk.TryLock()
	require.NoError(t, err)
	defer flk.Unlock() //nolint:errcheck
	nodes := []node.Type{node.Light, node.Bridge}

	for _, node := range nodes {
		cfg := DefaultConfig(node)
		require.Error(t, Init(*cfg, dir, node))
	}
}

// TestInit_generateNewKey tests to ensure new account is generated
// correctly.
func TestInit_generateNewKey(t *testing.T) {
	cfg := DefaultConfig(node.Bridge)

	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	ring, err := keyring.New(app.Name, cfg.State.DefaultBackendName, t.TempDir(), os.Stdin, encConf.Codec)
	require.NoError(t, err)

	originalKey, mn, err := generateNewKey(ring)
	require.NoError(t, err)

	// check ring and make sure it generated + stored key
	keys, err := ring.List()
	require.NoError(t, err)
	assert.Equal(t, originalKey, keys[0])

	// ensure the generated account is actually a celestia account
	addr, err := originalKey.GetAddress()
	require.NoError(t, err)
	assert.Contains(t, addr.String(), "celestia")

	// ensure account is recoverable from mnemonic
	ring2, err := keyring.New(app.Name, cfg.State.DefaultBackendName, t.TempDir(), os.Stdin, encConf.Codec)
	require.NoError(t, err)
	duplicateKey, err := ring2.NewAccount("test", mn, keyring.DefaultBIP39Passphrase, sdk.GetConfig().GetFullBIP44Path(),
		hd.Secp256k1)
	require.NoError(t, err)
	got, err := duplicateKey.GetAddress()
	require.NoError(t, err)
	assert.Equal(t, addr.String(), got.String())
}
