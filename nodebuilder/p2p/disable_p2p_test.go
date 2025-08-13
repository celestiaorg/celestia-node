package p2p

import (
	"testing"

	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/stretchr/testify/assert"
)

func TestDisableP2PModule(t *testing.T) {
	tests := []struct {
		name     string
		nodeType node.Type
		disableP2P bool
	}{
		{"Bridge with P2P enabled", node.Bridge, false},
		{"Bridge with P2P disabled", node.Bridge, true},
		{"Full with P2P enabled", node.Full, false},
		{"Full with P2P disabled", node.Full, true},
		{"Light with P2P enabled", node.Light, false},
		{"Light with P2P disabled", node.Light, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig(test.nodeType)
			cfg.DisableP2P = test.disableP2P

			// Test that the module can be constructed
			module := ConstructModule(test.nodeType, &cfg)
			assert.NotNil(t, module, "Module should be constructable")

			// Test that we can create an fx app with this module without panicking
			app := fx.New(
				module,
				fx.NopLogger,
			)
			assert.NotNil(t, app, "FX app should be created")
		})
	}
}