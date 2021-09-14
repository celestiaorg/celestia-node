package node

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/core"

	"github.com/celestiaorg/celestia-node/node/p2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

func TestNewFull(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Core.EmbeddedConfig = core.TestConfig(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.Core.EmbeddedConfig.RootDir)
	})

	nd, err := NewFull(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, nd)
	assert.NotNil(t, nd.Config)
	assert.NotZero(t, nd.Type)
}

func TestFullLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Core.EmbeddedConfig = core.TestConfig(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(cfg.Core.EmbeddedConfig.RootDir)
	})

	node, err := NewFull(cfg)
	assert.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotZero(t, node.Type)
	require.NotNil(t, node.CoreClient)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Stop(ctx)
	assert.NoError(t, err)
}

// TestFull_P2P_Streams tests the ability for Full nodes to communicate
// directly with each other via libp2p streams.
func TestFull_P2P_Streams(t *testing.T) {
	// create two embedded configs
	nodeEmbeddedConfig, peerEmbeddedConfig := core.TestConfig(t.Name()), core.TestConfig(t.Name())
	t.Cleanup(func() {
		os.RemoveAll(nodeEmbeddedConfig.RootDir)
		os.RemoveAll(peerEmbeddedConfig.RootDir)
	})

	// make first node
	nodeConf := DefaultConfig()
	nodeConf.Core.EmbeddedConfig = nodeEmbeddedConfig
	nodeConf.P2P = p2p.DefaultConfig()
	node, err := NewFull(nodeConf)
	assert.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Host)
	// make peer node
	peerConf := DefaultConfig()
	peerConf.Core.EmbeddedConfig = peerEmbeddedConfig
	peerConf.P2P = p2p.DefaultConfig()
	peerConf.P2P.ListenAddresses = []string{
		"/ip4/0.0.0.0/tcp/2124",
		"/ip6/::/tcp/2124",
	}
	peer, err := NewFull(peerConf)
	assert.NoError(t, err)
	require.NotNil(t, peer)
	require.NotNil(t, node.Host)

	nodeCtx, nodeCtxCancel := context.WithCancel(context.Background())
	peerCtx, peerCtxCancel := context.WithCancel(context.Background())
	t.Cleanup(nodeCtxCancel)
	t.Cleanup(peerCtxCancel)

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
	// stop both nodes
	require.NoError(t, node.Stop(nodeCtx))
	require.NoError(t, peer.Stop(peerCtx))
}
