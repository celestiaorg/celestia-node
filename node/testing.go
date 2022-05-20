package node

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/params"

	"github.com/celestiaorg/celestia-node/core"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	node, _, cfg := core.StartTestKVApp(ctx, t)
	opts = append(opts, WithRemoteCore(core.GetEndpoint(cfg)), WithNetwork(params.Private))
	store := MockStore(t, DefaultConfig(tp))
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}
