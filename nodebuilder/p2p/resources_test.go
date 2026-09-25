package p2p

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestResourceManager_AppliesOptions(t *testing.T) {
	allowed := ma.StringCast("/ip4/203.0.113.9/tcp/2121")

	limits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{Conns: 1, ConnsInbound: 1, ConnsOutbound: 1},
	}.Build(rcmgr.DefaultLimits.AutoScale())

	rm, err := resourceManager(resourceManagerParams{
		Limits: limits,
		Opts:   []rcmgr.Option{rcmgr.WithAllowlistedMultiaddrs([]ma.Multiaddr{allowed})},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rm.Close() })

	// take the only inbound connection slot
	other, err := rm.OpenConnection(network.DirInbound, true, ma.StringCast("/ip4/198.51.100.1/tcp/1"))
	require.NoError(t, err)
	t.Cleanup(other.Done)

	// an allowlisted address must still get in through the allowlisted scope
	scope, err := rm.OpenConnection(network.DirInbound, true, allowed)
	require.NoError(t, err)
	scope.Done()
}
