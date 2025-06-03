package p2p

import (
	"testing"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigDisableP2P(t *testing.T) {
	// Test that default config has p2p enabled
	cfg := DefaultConfig(node.Bridge)
	assert.False(t, cfg.DisableP2P, "Default config should have p2p enabled")
	
	// Test that we can disable p2p
	cfg.DisableP2P = true
	assert.True(t, cfg.DisableP2P, "Should be able to disable p2p")
}

func TestDisableP2PForDifferentNodeTypes(t *testing.T) {
	nodeTypes := []node.Type{node.Bridge, node.Full, node.Light}
	
	for _, tp := range nodeTypes {
		cfg := DefaultConfig(tp)
		assert.False(t, cfg.DisableP2P, "Default config for %s should have p2p enabled", tp)
		
		// Test that we can disable p2p for any node type
		cfg.DisableP2P = true
		assert.True(t, cfg.DisableP2P, "Should be able to disable p2p for %s", tp)
	}
}