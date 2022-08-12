package node

import (
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/node/config"
	"github.com/celestiaorg/celestia-node/params"
)

func TestNewLightWithP2PKey(t *testing.T) {
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	node := TestNode(t, config.Light, config.WithP2PKey(key))
	assert.True(t, node.Host.ID().MatchesPrivateKey(key))
}

func TestNewLightWithHost(t *testing.T) {
	nw, _ := mocknet.WithNPeers(1)
	node := TestNode(t, config.Light, config.WithHost(nw.Hosts()[0]))
	assert.Equal(t, nw.Peers()[0], node.Host.ID())
}

func TestLight_WithMutualPeers(t *testing.T) {
	peers := []string{
		"/ip6/100:0:114b:abc5:e13a:c32f:7a9e:f00a/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
		"/ip4/192.168.1.10/tcp/2121/p2p/12D3KooWSRqDfpLsQxpyUhLC9oXHD2WuZ2y5FWzDri7LT4Dw9fSi",
	}
	node := TestNode(t, config.Light, config.WithMutualPeers(peers))
	require.NotNil(t, node)
	assert.Equal(t, node.Config.P2P.MutualPeers, peers)
}

func TestLight_WithNetwork(t *testing.T) {
	node := TestNode(t, config.Light)
	require.NotNil(t, node)
	assert.Equal(t, params.Private, node.Network)
}
