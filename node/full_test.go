package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	handlers "github.com/celestiaorg/celestia-node/node/rpc/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

func TestNewFull(t *testing.T) {
	repo := MockRepository(t, DefaultConfig(Full))
	node, err := New(Full, repo)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotNil(t, node.Host)
	assert.NotZero(t, node.Type)
}

func TestFullLifecycle(t *testing.T) {
	repo := MockRepository(t, DefaultConfig(Full))
	node, err := New(Full, repo)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.RPCServer)
	require.NotNil(t, node.Config)
	require.NotZero(t, node.Type)
	require.NotNil(t, node.Host)
	require.NotNil(t, node.CoreClient)
	require.NotNil(t, node.BlockServ)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Stop(ctx)
	require.NoError(t, err)
}

// TestFull_P2P_Streams tests the ability for Full nodes to communicate
// directly with each other via libp2p streams.
func TestFull_P2P_Streams(t *testing.T) {
	repo := MockRepository(t, DefaultConfig(Full))
	node, err := New(Full, repo)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Host)

	peerConf := DefaultConfig(Full)
	// modify address to be different
	peerConf.P2P.ListenAddresses = []string{
		"/ip4/0.0.0.0/tcp/2124",
		"/ip6/::/tcp/2124",
	}
	repo = MockRepository(t, peerConf)
	peer, err := New(Full, repo)
	require.NoError(t, err)
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

func TestFull_RPCServer(t *testing.T) {
	repo := MockRepository(t, DefaultConfig(Full))
	node, err := New(Full, repo)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Host)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	// register simple ping handler on node's server
	err = node.RegisterAPI("/ping", ping{})
	require.NoError(t, err)
	// send ping
	resp, err := http.Get(fmt.Sprintf("http://%s/ping", node.Config.RPC.ListenAddr))
	require.NoError(t, err)
	// read pong
	pong, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close()
	})
	assert.Equal(t, "pong", string(pong))

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, node.Stop(ctx))
}

type ping struct{}

func (p ping) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//nolint:errcheck
	w.Write([]byte("pong"))
}

func TestFull_RPCStatus(t *testing.T) {
	repo := MockRepository(t, DefaultConfig(Full))
	node, err := New(Full, repo)
	require.NoError(t, err)
	require.NotNil(t, node)
	require.NotNil(t, node.Host)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = node.Start(ctx)
	require.NoError(t, err)

	// expected result
	expectedStatus, err := json.Marshal(handlers.StatusHandler{
		Address: node.Config.P2P.ListenAddresses,
		Network: node.Config.P2P.Network,
	})
	require.NoError(t, err)

	// get status
	resp, err := http.Get(fmt.Sprintf("http://%s/status", node.Config.RPC.ListenAddr))
	require.NoError(t, err)

	// read status
	actualStatus, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Cleanup(func() {
		resp.Body.Close()
	})
	assert.Equal(t, expectedStatus, actualStatus)

	ctx, cancel = context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, node.Stop(ctx))
}
