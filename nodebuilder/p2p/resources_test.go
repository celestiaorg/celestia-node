package p2p

import (
	"context"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/test"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
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

func TestAllowList_BindsPeerID(t *testing.T) {
	addr := ma.StringCast("/ip4/203.0.113.9/tcp/2121")
	mutual := test.RandPeerIDFatal(t)
	bootstrapper := test.RandPeerIDFatal(t)

	cfg := DefaultConfig(node.Light)
	cfg.MutualPeers = []string{addr.String() + "/p2p/" + mutual.String()}
	bootstrappers := Bootstrappers{{ID: bootstrapper, Addrs: []ma.Multiaddr{addr}}}

	opt, err := allowList(context.Background(), &cfg, bootstrappers)
	require.NoError(t, err)
	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()), opt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rm.Close() })

	al := rcmgr.GetAllowlist(rm)
	require.True(t, al.AllowedPeerAndMultiaddr(mutual, addr))
	require.True(t, al.AllowedPeerAndMultiaddr(bootstrapper, addr))
	require.False(t, al.AllowedPeerAndMultiaddr(test.RandPeerIDFatal(t), addr))
}

func TestAllowList_ResolvesDNSAddr(t *testing.T) {
	bootstrapper := test.RandPeerIDFatal(t)
	other := test.RandPeerIDFatal(t)

	// a /dnsaddr record resolves to /dns addresses, as on mainnet
	resolver, err := madns.NewResolver(madns.WithDefaultResolver(&madns.MockResolver{
		TXT: map[string][]string{"_dnsaddr.boot.example": {
			"dnsaddr=/dns/boot.example/tcp/2121/p2p/" + bootstrapper.String(),
			"dnsaddr=/dns/boot.example/tcp/2121/p2p/" + other.String(),
		}},
		IP: map[string][]net.IPAddr{"boot.example": {{IP: net.ParseIP("203.0.113.9")}}},
	}))
	require.NoError(t, err)
	defaultResolver := madns.DefaultResolver
	madns.DefaultResolver = resolver
	t.Cleanup(func() { madns.DefaultResolver = defaultResolver })

	cfg := DefaultConfig(node.Light)
	bootstrappers := Bootstrappers{{ID: bootstrapper, Addrs: []ma.Multiaddr{ma.StringCast("/dnsaddr/boot.example")}}}

	opt, err := allowList(context.Background(), &cfg, bootstrappers)
	require.NoError(t, err)
	rm, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.DefaultLimits.AutoScale()), opt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rm.Close() })

	addr := ma.StringCast("/ip4/203.0.113.9/tcp/2121")
	al := rcmgr.GetAllowlist(rm)
	require.True(t, al.AllowedPeerAndMultiaddr(bootstrapper, addr))
	require.False(t, al.AllowedPeerAndMultiaddr(other, addr))
}
