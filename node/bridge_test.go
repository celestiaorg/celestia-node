package node

import (
	"context"
	"testing"

	"github.com/celestiaorg/celestia-node/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

func TestNewBridge(t *testing.T) {
	store := MockStore(t, DefaultConfig(Bridge))
	node, err := New(Bridge, store)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotNil(t, node.Host)
	require.NotNil(t, node.HeaderServ)
	require.Nil(t, node.BlockServ)
	assert.NotZero(t, node.Type)
}

func TestBridgeLifecycle(t *testing.T) {
	cfg := DefaultConfig(Bridge)
	store := MockStore(t, cfg)

	node, err := New(Bridge, store)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotZero(t, node.Type)
	require.NotNil(t, node.Host)
	require.NotNil(t, node.CoreClient)
	require.NotNil(t, node.HeaderServ)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Stop(ctx)
	require.NoError(t, err)
}

// TestBridge_P2P_Streams tests the ability for Bridge nodes to communicate
// directly with each other via libp2p streams.
func TestBridge_P2P_Streams(t *testing.T) {
	store := MockStore(t, DefaultConfig(Bridge))
	node, err := New(Bridge, store)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Host)

	peerConf := DefaultConfig(Bridge)
	// modify address to be different
	peerConf.P2P.ListenAddresses = []string{
		"/ip4/0.0.0.0/tcp/2124",
		"/ip6/::/tcp/2124",
	}
	store = MockStore(t, peerConf)
	peer, err := New(Bridge, store)
	require.NoError(t, err)
	require.NotNil(t, peer)
	require.NotNil(t, peer.Host)

	nodeCtx, nodeCtxCancel := context.WithCancel(context.Background())
	t.Cleanup(nodeCtxCancel)

	// connect node to peer
	peerAddrID := host.InfoFromHost(peer.Host)
	err = node.Host.Connect(nodeCtx, *peerAddrID)
	require.NoError(t, err)
	// create handler to read from conn on peer side
	protocolID := protocol.ID("test")
	peer.Host.SetStreamHandler(protocolID, func(stream network.Stream) {
		buf := make([]byte, 5)
		_, err := stream.Read(buf)
		require.NoError(t, err)
		require.Equal(t, "hello", string(buf))
	})
	// open a stream between node and peer
	stream, err := node.Host.NewStream(nodeCtx, peer.Host.ID(), protocolID)
	require.NoError(t, err)
	// write hello to peer
	_, err = stream.Write([]byte("hello"))
	require.NoError(t, err)

	// stop the connection
	require.NoError(t, stream.Close())
}

func TestBridge_WithRemoteCore(t *testing.T) {
	// TODO(@Wondertan): Fix core
	t.Skip("Skip until we fix core")

	store := MockStore(t, DefaultConfig(Bridge))
	remoteCore, protocol, ip := core.StartRemoteCore()
	t.Cleanup(func() {
		remoteCore.Stop() // nolint:errcheck
	})
	require.NotNil(t, remoteCore)
	assert.True(t, remoteCore.IsRunning())

	node, err := New(Bridge, store, WithRemoteCore(protocol, ip))
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.True(t, node.CoreClient.IsRunning())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Stop(ctx)
	require.NoError(t, err)
	err = remoteCore.Stop()
	require.NoError(t, err)
}

func TestBridge_NotPanicWithNilOpts(t *testing.T) {
	store := MockStore(t, DefaultConfig(Bridge))
	node, err := New(Bridge, store, nil)
	require.NoError(t, err)
	require.NotNil(t, node)
}

func TestBridge_WithMockedCoreClient(t *testing.T) {
	repo := MockStore(t, DefaultConfig(Bridge))
	node, err := New(Bridge, repo, WithCoreClient(core.MockEmbeddedClient()))
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.True(t, node.CoreClient.IsRunning())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	err = node.Stop(ctx)
	require.NoError(t, err)
}

func TestBridge_WithMutualPeers(t *testing.T) {
	repo := MockStore(t, DefaultConfig(Bridge))
	peers := []string{
		"/ip6/100:0:114b:abc5:e13a:c32f:7a9e:f00a/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
		"/ip4/192.168.1.10/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
	}
	node, err := New(Bridge, repo, WithMutualPeers(peers))
	require.NoError(t, err)
	require.NotNil(t, node)

	assert.Equal(t, node.Config.P2P.MutualPeers, peers)
}

func TestBridge_WithBootstrapPeers(t *testing.T) {
	repo := MockStore(t, DefaultConfig(Bridge))
	peers := []string{
		"/ip6/100:0:114b:abc5:e13a:c32f:7a9e:f00a/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
		"/ip4/192.168.1.10/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
	}
	node, err := New(Bridge, repo, WithBootstrapPeers(peers))
	require.NoError(t, err)
	require.NotNil(t, node)

	assert.Equal(t, node.Config.P2P.BootstrapPeers, peers)
}
