package node

import (
	"testing"

	"github.com/stretchr/testify/require"

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
	remote, _, err := core.StartRemoteClient()
	require.NoError(t, err)
	t.Cleanup(func() {
		remote.Stop() //nolint:errcheck
	})

	cfg := DefaultConfig(tp)
	cfg.Core.Protocol, cfg.Core.RemoteAddr = core.GetRemoteEndpoint(remote)

	store := MockStore(t, cfg)
	nd, err := New(tp, store, opts...)
	require.NoError(t, err)
	return nd
}
