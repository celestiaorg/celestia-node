package node

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/node/fxutil"
)

const testStr = "testString"

var initiallyEmpty = ""

func TestPluginComponentCall(t *testing.T) {
	store := MockStore(t, DefaultConfig(Light))
	nodes := []Type{Light, Bridge}
	plug := &testPlugin{}
	for i, node := range nodes {
		_, err := New(node, store, WithPlugins(plug))
		require.NoError(t, err)
		assert.Greater(t, plug.counter, i)
	}
}

func TestPluginProvide(t *testing.T) {
	store := MockStore(t, DefaultConfig(Light))
	plug := &testPlugin{}
	_, err := New(Light, store, WithPlugins(plug))
	require.NoError(t, err)

	assert.Equal(t, testStr, initiallyEmpty)
}

func TestPluginInit(t *testing.T) {
	dir := t.TempDir()
	nodes := []Type{Light, Bridge}
	plugin := &testPlugin{}
	for _, node := range nodes {
		require.NoError(t, Init(dir, node, WithPlugins(plugin)))
		assert.Equal(t, dir, plugin.path)
	}
}

func TestMultiplePlugins(t *testing.T) {
	p1 := &testPlugin{}
	p2 := &testPlugin2{&testPlugin{}}
	store := MockStore(t, DefaultConfig(Light))
	nodes := []Type{Light, Bridge}
	for i, node := range nodes {
		_, err := New(node, store, WithPlugins(p1, p2))
		require.NoError(t, err)
		assert.Greater(t, p1.counter, i)
		assert.Greater(t, p2.counter, i)
	}
	assert.Equal(t, testStr, initiallyEmpty)
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
	fmt.Println("calling components")
	plug.counter++
	return fxutil.Raw(
		fx.Provide(
			fx.Annotate(
				provider,
				fx.ResultTags(`group:"plugins"`),
			),
		),
	)
}

// use the PluginOutlet to force fx to call this function
func provider() PluginOutlet {
	initiallyEmpty = testStr
	return struct{}{}
}

type testPlugin2 struct {
	*testPlugin
}
