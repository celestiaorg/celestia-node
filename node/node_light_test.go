//go:build test_unit

package node

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/params"
)

func TestNewLightAndLifecycle(t *testing.T) {
	node := TestNode(t, Light)
	require.NotNil(t, node)
	require.NotNil(t, node.Config)
	require.NotNil(t, node.HeaderServ)
	assert.NotZero(t, node.Type)

	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := node.Start(startCtx)
	require.NoError(t, err)

	stopCtx, stopCtxCancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		stopCtxCancel()
	})

	err = node.Stop(stopCtx)
	require.NoError(t, err)
}

func TestNewLightWithP2PKey(t *testing.T) {
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	node := TestNode(t, Light, WithP2PKey(key))
	assert.True(t, node.Host.ID().MatchesPrivateKey(key))
}

func TestNewLightWithHost(t *testing.T) {
	nw, _ := mocknet.WithNPeers(context.Background(), 1)
	node := TestNode(t, Light, WithHost(nw.Host(nw.Peers()[0])))
	assert.Equal(t, node.Host.ID(), nw.Peers()[0])
}

func TestLight_WithMutualPeers(t *testing.T) {
	peers := []string{
		"/ip6/100:0:114b:abc5:e13a:c32f:7a9e:f00a/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
		"/ip4/192.168.1.10/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
	}
	node := TestNode(t, Light, WithMutualPeers(peers))
	require.NotNil(t, node)
	assert.Equal(t, node.Config.P2P.MutualPeers, peers)
}

func TestLight_WithNetwork(t *testing.T) {
	node := TestNode(t, Light, WithNetwork(params.Private))
	require.NotNil(t, node)
	assert.Equal(t, node.Network, params.Private)
}
