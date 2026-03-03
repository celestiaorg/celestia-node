package tests
package nodebuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
)

// TestRDANodeService ensures RDA service is properly initialized
func TestRDANodeService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping RDA integration test in short mode")
	}

	// Create node with RDA enabled
	cfg := DefaultConfig(node.Light)
	cfg.Share.RDA = &share.RDAConfig{
		Enabled:           true,
		ExpectedNodeCount: 1000,
		GridRows:          0, // auto-calculate
		GridCols:          0, // auto-calculate
		FilterPolicy:      "default",
		MaxRowPeers:       256,
		MaxColPeers:       256,
	}

	ctx := context.Background()
	store := NewTestStore(t)
	defer store.Close()

	// Create test node
	nd, err := NewWithConfig(node.Light, testNetwork, store, cfg)
	require.NoError(t, err)
	require.NotNil(t, nd)

	// Start node
	err = nd.Start(ctx)
	require.NoError(t, err)
	defer nd.Stop(ctx) //nolint:errcheck

	// Verify RDA service exists if enabled
	if cfg.Share.RDA.Enabled {
		require.NotNil(t, nd.RDAService, "RDA service should be initialized")
		require.NotNil(t, nd.RDAService.GetGridManager(), "Grid manager should be initialized")
















































































































































































}	require.NotNil(t, nd.ShareServ)	require.NotNil(t, nd)	// Node should work fine without RDA	defer nd.Stop(ctx) //nolint:errcheck	require.NoError(t, err)	err = nd.Start(ctx)	require.NoError(t, err)	nd, err := NewWithConfig(node.Light, testNetwork, store, cfg)	defer store.Close()	store := NewTestStore(t)	ctx := context.Background()	}		Enabled: false,	cfg.Share.RDA = &share.RDAConfig{	cfg := DefaultConfig(node.Light)	}		t.Skip("Skipping RDA disabled test in short mode")	if testing.Short() {func TestRDADisabled(t *testing.T) {// TestRDADisabled tests that node works without RDA enabled}	}		})			}				require.Error(t, err, "Configuration should be invalid")			} else {				require.NoError(t, err, "Configuration should be valid")			if tt.isValid {			err := tt.config.Validate()		t.Run(tt.name, func(t *testing.T) {	for _, tt := range tests {	}		},			isValid: false,			},				MaxColPeers:       256,				MaxRowPeers:       256,				FilterPolicy:      "invalid_policy",				GridCols:          128,				GridRows:          128,				ExpectedNodeCount: 10000,				Enabled:           true,			config: &share.RDAConfig{			name: "invalid_filter_policy",		{		},			isValid: false,			},				MaxColPeers:       16,				MaxRowPeers:       16, // Too small				FilterPolicy:      "default",				GridCols:          128,				GridRows:          128,				ExpectedNodeCount: 10000,				Enabled:           true,			config: &share.RDAConfig{			name: "invalid_max_peers_too_small",		{		},			isValid: false,			},				MaxColPeers:       256,				MaxRowPeers:       256,				FilterPolicy:      "default",				GridCols:          128,				GridRows:          128,				ExpectedNodeCount: 50, // Too small				Enabled:           true,			config: &share.RDAConfig{			name: "invalid_node_count_too_small",		{		},			isValid: true,			},				MaxColPeers:       256,				MaxRowPeers:       256,				FilterPolicy:      "default",				GridCols:          128,				GridRows:          128,				ExpectedNodeCount: 10000,				Enabled:           true,			config: &share.RDAConfig{			name: "valid_config",		{	}{		isValid bool		config  *share.RDAConfig		name    string	tests := []struct {func TestRDAConfiguration(t *testing.T) {// TestRDAConfiguration tests that RDA configuration is properly validated}	require.Greater(t, defaultPolicy.MaxColPeers, 0, "MaxColPeers should be positive")	require.Greater(t, defaultPolicy.MaxRowPeers, 0, "MaxRowPeers should be positive")	require.True(t, defaultPolicy.AllowColCommunication, "Default policy should allow col communication")	require.True(t, defaultPolicy.AllowRowCommunication, "Default policy should allow row communication")	defaultPolicy := share.DefaultFilterPolicy()	// Verify default policy settings	require.NotNil(t, filter)	filter := share.NewPeerFilter(nil, gridMgr, share.DefaultFilterPolicy())	// Create filter with default policy	gridMgr := share.NewRDAGridManager(share.DefaultGridDimensions)	}		t.Skip("Skipping RDA peer filtering test in short mode")	if testing.Short() {func TestRDAPeerFiltering(t *testing.T) {// TestRDAPeerFiltering tests that peer filtering works correctly}	require.Greater(t, int(dims.Cols), 0, "Cols should be positive")	require.Greater(t, int(dims.Rows), 0, "Rows should be positive")	dims := gridMgr.GetGridDimensions()	// Verify positions are within grid bounds	}		_ = testID		// In real test, use actual peer IDs from libp2p		// Mock peer registration would happen here		testID := "test_peer_" + string(rune(i))	for i := 0; i < 10; i++ {	peerIDs := make(map[string]share.GridPosition)	// Register multiple peers	gridMgr := share.NewRDAGridManager(share.DefaultGridDimensions)	}		t.Skip("Skipping RDA peer position test in short mode")	if testing.Short() {func TestRDAPeerPosition(t *testing.T) {// TestRDAPeerPosition tests that peers get consistent grid positions}	}		})			require.Less(t, int(dims.Cols), tt.maxDim, "Cols should be < maxDim")			require.Greater(t, int(dims.Cols), tt.minDim, "Cols should be >= minDim")			require.Less(t, int(dims.Rows), tt.maxDim, "Rows should be < maxDim")			require.Greater(t, int(dims.Rows), tt.minDim, "Rows should be >= minDim")			// Verify grid dimensions are reasonable						dims := share.CalculateOptimalGridSize(tt.expectedNodes)		t.Run("", func(t *testing.T) {	for _, tt := range tests {	}		{10000, 95, 105},     // ~10000 nodes -> ~100x100 grid		{1000, 30, 34},       // ~1000 nodes -> ~32x32 grid		{100, 10, 12},        // ~100 nodes -> ~10x10 grid	}{		maxDim        int		minDim        int		expectedNodes uint32	tests := []struct {	// Test cases for different node counts	}		t.Skip("Skipping RDA grid test in short mode")	if testing.Short() {func TestRDAGridDimensions(t *testing.T) {// TestRDAGridDimensions tests that grid dimensions are properly calculated}	}		require.NotNil(t, nd.RDAService.GetPeerManager(), "Peer manager should be initialized")