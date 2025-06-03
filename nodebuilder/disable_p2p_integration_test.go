package nodebuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func TestStorageOnlyBridgeNode(t *testing.T) {
	cfg := DefaultConfig(node.Bridge)
	
	// Enable storage-only mode by disabling p2p
	cfg.P2P.DisableP2P = true
	
	// Create a minimal store for testing
	memStore := NewMemStore()
	
	// Test that the module can be constructed with p2p disabled
	module := ConstructModule(node.Bridge, p2p.Mocha, cfg, memStore)
	require.NotNil(t, module, "Module should be constructable with p2p disabled")
	
	// Test that an fx app can be created without panicking
	// This validates dependency resolution
	var app *fx.App
	require.NotPanics(t, func() {
		app = fx.New(
			module,
			fx.NopLogger,
			fx.Invoke(func() {
				// Empty invoke function to test basic startup
			}),
		)
	}, "FX app creation should not panic with p2p disabled")
	
	require.NotNil(t, app, "FX app should be created")
	
	// Test starting and stopping the app context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start in background and immediately stop to test initialization
	err := app.Start(ctx)
	assert.NoError(t, err, "App should start successfully with p2p disabled")
	
	err = app.Stop(ctx)
	assert.NoError(t, err, "App should stop successfully")
}