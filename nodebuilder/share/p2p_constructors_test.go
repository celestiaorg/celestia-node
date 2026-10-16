package share

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/shwap/p2p/discovery"
)

func TestLimitArchivalPeers(t *testing.T) {
	params := discovery.Parameters{PeersLimit: 10}
	limited := limitArchivalPeers(params)
	require.Equal(t, uint(5), limited.PeersLimit)
	require.Equal(t, uint(10), params.PeersLimit)

	params.PeersLimit = 3
	limited = limitArchivalPeers(params)
	require.Equal(t, uint(3), limited.PeersLimit)
}
