package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

const testStr = "testString"

var initiallyEmpty = ""

func TestPluginComponentCall(t *testing.T) {
	store := MockStore(t, DefaultConfig(Light))
	nodes := []Type{Light, Bridge}
	componentPlugin := &testPlugin{}
	for i, node := range nodes {
		_, err := New(node, store, []Plugin{componentPlugin})
		require.NoError(t, err)
		assert.Greater(t, componentPlugin.counter, i)
	}
}

func TestPluginProvide(t *testing.T) {
	store := MockStore(t, DefaultConfig(Light))
	componentPlugin := &testPlugin{}
	_, err := New(Light, store, []Plugin{componentPlugin})
	require.NoError(t, err)

	assert.Equal(t, testStr, initiallyEmpty)
}

func TestPluginInit(t *testing.T) {
	dir := t.TempDir()
	nodes := []Type{Light, Bridge}
	plugin := &testPlugin{}
	for _, node := range nodes {
		require.NoError(t, Init(dir, node, []Plugin{plugin}))
		assert.Equal(t, dir, plugin.path)
	}
}

type testPlugin struct {
	counter int
	path    string
}

func (plug *testPlugin) Name() string { return "test" }
func (plug *testPlugin) Initialize(path string) error {
	plug.path = path
	return nil
}
func (plug *testPlugin) Components(cfg *Config, store Store) fxutil.Option {
	plug.counter++
	return fxutil.Options(fxutil.Provide(provider))
}

// use the PluginOutlet to force fx to call this function
func provider() PluginOutlet {
	initiallyEmpty = testStr
	return struct{}{}
}
