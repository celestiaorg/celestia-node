package node

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/params"
)

// MockStore provides mock in memory Store for testing purposes.
func MockStore(t *testing.T, cfg *Config) Store {
	t.Helper()
	store := NewMemStore()
	err := store.PutConfig(cfg)
	require.NoError(t, err)
	return store
}

func TestNode(t *testing.T, tp Type, opts ...Option) *Node {
	node, _ := core.StartTestKVApp(t)
	opts = append(opts, WithRemoteCore(core.GetEndpoint(node)), WithNetwork(params.Private))
	store := MockStore(t, DefaultConfig(tp))
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}
