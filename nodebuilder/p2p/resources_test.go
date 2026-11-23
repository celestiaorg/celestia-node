package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// TestBridgeResourcesAdmitConnManagerHigh checks that the resource manager admits at least
// ConnManager.High connections, so connmgr trims unprotected peers before rcmgr rejects.
func TestBridgeResourcesAdmitConnManagerHigh(t *testing.T) {
	cfg := DefaultConfig(node.Bridge)
	limits := bridgeResources(&cfg, Mainnet).ToPartialLimitConfig().System
	high := cfg.ConnManager.High

	require.GreaterOrEqual(t, int(limits.Conns), high)
	require.GreaterOrEqual(t, int(limits.ConnsInbound), high)
	require.GreaterOrEqual(t, int(limits.ConnsOutbound), high)
}
