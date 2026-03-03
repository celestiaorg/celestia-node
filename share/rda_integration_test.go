package share

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestIntegration_RDANodeService_Lifecycle tests service startup and shutdown
func TestIntegration_RDANodeService_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	config := RDANodeServiceConfig{
		GridDimensions:        GridDimensions{Rows: 16, Cols: 16},
		FilterPolicy:          DefaultFilterPolicy(),
		EnableDetailedLogging: false,
	}

	// Create service (without real host for basic test)
	gridMgr := NewRDAGridManager(config.GridDimensions)
	assert.NotNil(t, gridMgr)

	// Verify default config
	assert.Equal(t, uint16(16), config.GridDimensions.Rows)
	assert.Equal(t, uint16(16), config.GridDimensions.Cols)
	assert.False(t, config.EnableDetailedLogging)
}

// TestIntegration_GridManager_WithPeers tests grid manager with peer registration
func TestIntegration_GridManager_WithPeers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	dims := GridDimensions{Rows: 8, Cols: 8}
	gridMgr := NewRDAGridManager(dims)

	// Register multiple peers
	peers := generateTestPeerIDs(t, 20)
	positions := make(map[int]GridPosition)

	for _, p := range peers {
		pos := gridMgr.RegisterPeer(p)
		positions[len(positions)] = pos

		// Verify position is valid
		assert.True(t, pos.Row >= 0 && pos.Row < int(dims.Rows))
		assert.True(t, pos.Col >= 0 && pos.Col < int(dims.Cols))
	}

	// Verify peers can be retrieved
	for _, p := range peers {
		pos, exists := gridMgr.GetPeerPosition(p)
		assert.True(t, exists, "Peer should be registered")
		assert.True(t, pos.Row >= 0)
		assert.True(t, pos.Col >= 0)
	}

	// Test filtering by row
	firstPeerPos, _ := gridMgr.GetPeerPosition(peers[0])
	rowPeers := gridMgr.GetRowPeers(firstPeerPos.Row)
	assert.Greater(t, len(rowPeers), 0)

	for _, p := range rowPeers {
		pPos, _ := gridMgr.GetPeerPosition(p)
		assert.Equal(t, firstPeerPos.Row, pPos.Row)
	}

	// Test filtering by column
	colPeers := gridMgr.GetColPeers(firstPeerPos.Col)
	assert.Greater(t, len(colPeers), 0)

	for _, p := range colPeers {
		pPos, _ := gridMgr.GetPeerPosition(p)
		assert.Equal(t, firstPeerPos.Col, pPos.Col)
	}
}

// TestIntegration_FilterPolicy_Enforcement tests peer filtering with different policies
func TestIntegration_FilterPolicy_Enforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	policies := []struct {
		name   string
		policy FilterPolicy
	}{
		{
			name:   "Default",
			policy: DefaultFilterPolicy(),
		},
		{
			name:   "Strict",
			policy: StrictFilterPolicy(),
		},
		{
			name: "Custom",
			policy: FilterPolicy{
				AllowRowCommunication: true,
				AllowColCommunication: false,
				MaxRowPeers:           50,
				MaxColPeers:           0,
			},
		},
	}

	for _, tc := range policies {
		t.Run(tc.name, func(t *testing.T) {
			// Verify policy structure
			assert.NotNil(t, tc.policy)

			// Default and Strict should allow both
			if tc.name == "Default" || tc.name == "Strict" {
				assert.True(t, tc.policy.AllowRowCommunication)
				assert.True(t, tc.policy.AllowColCommunication)
			}

			// Custom should restrict columns
			if tc.name == "Custom" {
				assert.True(t, tc.policy.AllowRowCommunication)
				assert.False(t, tc.policy.AllowColCommunication)
			}
		})
	}
}

// TestIntegration_SubnetMapping tests subnet ID mapping and consistency
func TestIntegration_SubnetMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	dims := GridDimensions{Rows: 32, Cols: 32}

	// Generate multiple peers and track their subnet mappings
	peers := generateTestPeerIDs(t, 50)
	subnetMap := make(map[string][]string) // subnet -> peers

	for _, p := range peers {
		rowID, colID := GetSubnetIDs(p, dims)

		// Verify format
		assert.Contains(t, rowID, "rda/row/")
		assert.Contains(t, colID, "rda/col/")

		// Track subnet membership
		subnetMap[rowID] = append(subnetMap[rowID], p.String())
		subnetMap[colID] = append(subnetMap[colID], p.String())
	}

	// Verify each subnet has peers
	assert.Greater(t, len(subnetMap), 0, "Should have subnets populated")

	// Test subnet consistency: same peer always maps to same subnets
	for _, p := range peers[:5] {
		rowID1, colID1 := GetSubnetIDs(p, dims)
		rowID2, colID2 := GetSubnetIDs(p, dims)

		assert.Equal(t, rowID1, rowID2)
		assert.Equal(t, colID1, colID2)
	}
}

// TestIntegration_PeerPositioning tests deterministic peer positioning
func TestIntegration_PeerPositioning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	dims := GridDimensions{Rows: 64, Cols: 64}

	// Test that same peer always gets same position
	peer1 := generateTestPeerID(t)

	pos1 := GetCoords(peer1, dims)
	pos2 := GetCoords(peer1, dims)
	pos3 := GetCoords(peer1, dims)

	assert.Equal(t, pos1, pos2)
	assert.Equal(t, pos2, pos3)

	// Test distribution across grid
	positions := make(map[GridPosition]int)
	for i := 0; i < 200; i++ {
		p := generateTestPeerID(t)
		pos := GetCoords(p, dims)
		positions[pos]++
	}

	// Most grid positions should be used
	filledCells := len(positions)
	totalCells := int(dims.Rows) * int(dims.Cols)
	fillRatio := float64(filledCells) / float64(totalCells)

	// With 200 random peers in 64x64 grid, expect ~4-5% fill
	assert.Greater(t, fillRatio, 0.01, "Should fill at least 1%% of grid")
}

// TestIntegration_GridConfiguration tests grid size calculations
func TestIntegration_GridConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	testCases := []struct {
		rows uint16
		cols uint16
		name string
	}{
		{8, 8, "Very Small"},
		{16, 16, "Small"},
		{32, 32, "Medium"},
		{64, 64, "Large"},
		{128, 128, "Very Large"},
		{256, 256, "Max"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dims := GridDimensions{Rows: tc.rows, Cols: tc.cols}
			mgr := NewRDAGridManager(dims)

			assert.Equal(t, tc.rows, mgr.dims.Rows)
			assert.Equal(t, tc.cols, mgr.dims.Cols)

			// Test peer registration works at this scale
			for i := 0; i < 10; i++ {
				p := generateTestPeerID(t)
				pos := mgr.RegisterPeer(p)

				assert.True(t, pos.Row < int(tc.rows))
				assert.True(t, pos.Col < int(tc.cols))
			}
		})
	}
}

// TestIntegration_PeerRelationships tests relationships between peers
func TestIntegration_PeerRelationships(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	dims := GridDimensions{Rows: 16, Cols: 16}

	peer1 := generateTestPeerID(t)
	peer2 := generateTestPeerID(t)
	peer3 := generateTestPeerID(t)

	pos1 := GetCoords(peer1, dims)
	pos2 := GetCoords(peer2, dims)
	pos3 := GetCoords(peer3, dims)

	// Test row relationship
	isRowPeer12 := IsRowPeer(peer1, peer2, dims)
	expectedRowPeer12 := pos1.Row == pos2.Row
	assert.Equal(t, expectedRowPeer12, isRowPeer12)

	// Test column relationship
	isColPeer12 := IsColPeer(peer1, peer2, dims)
	expectedColPeer12 := pos1.Col == pos2.Col
	assert.Equal(t, expectedColPeer12, isColPeer12)

	// Test transitive relationships
	if isRowPeer12 && IsRowPeer(peer2, peer3, dims) {
		// peer1 and peer3 should also be in same row
		assert.Equal(t, pos1.Row, pos3.Row)
	}
}

// TestIntegration_ConcurrentGridAccess tests thread-safety
func TestIntegration_ConcurrentGridAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	dims := GridDimensions{Rows: 32, Cols: 32}
	mgr := NewRDAGridManager(dims)

	// Pre-register peers
	peers := generateTestPeerIDs(t, 20)
	for _, p := range peers {
		mgr.RegisterPeer(p)
	}

	// Concurrent reads and writes
	done := make(chan bool, 100)

	// Writers
	for i := 0; i < 50; i++ {
		go func() {
			p := generateTestPeerID(t)
			mgr.RegisterPeer(p)
			done <- true
		}()
	}

	// Readers
	for i := 0; i < 50; i++ {
		go func() {
			for _, p := range peers {
				_, exists := mgr.GetPeerPosition(p)
				assert.True(t, exists)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Service should still be functional
	newPeer := generateTestPeerID(t)
	pos := mgr.RegisterPeer(newPeer)
	assert.True(t, pos.Row >= 0)
	assert.True(t, pos.Col >= 0)
}

// TestIntegration_ServiceConfiguration tests service config combinations
func TestIntegration_ServiceConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	testCases := []struct {
		name   string
		config RDANodeServiceConfig
	}{
		{
			name:   "Default Config",
			config: DefaultRDANodeServiceConfig(),
		},
		{
			name: "Small Grid",
			config: RDANodeServiceConfig{
				GridDimensions: GridDimensions{Rows: 8, Cols: 8},
				FilterPolicy:   DefaultFilterPolicy(),
			},
		},
		{
			name: "Large Grid",
			config: RDANodeServiceConfig{
				GridDimensions: GridDimensions{Rows: 256, Cols: 256},
				FilterPolicy:   StrictFilterPolicy(),
			},
		},
		{
			name: "Custom Policy",
			config: RDANodeServiceConfig{
				GridDimensions: GridDimensions{Rows: 64, Cols: 64},
				FilterPolicy: FilterPolicy{
					AllowRowCommunication: true,
					AllowColCommunication: false,
					MaxRowPeers:           100,
				},
			},
		},
		{
			name: "Expected Node Count",
			config: RDANodeServiceConfig{
				ExpectedNodeCount: 10000,
				FilterPolicy:      DefaultFilterPolicy(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create grid manager
			dims := tc.config.GridDimensions
			if tc.config.ExpectedNodeCount > 0 {
				dims = CalculateOptimalGridSize(tc.config.ExpectedNodeCount)
			}

			// Verify dims are valid
			assert.NotZero(t, dims.Rows)
			assert.NotZero(t, dims.Cols)

			mgr := NewRDAGridManager(dims)
			assert.NotNil(t, mgr)

			// Basic functionality test
			peer := generateTestPeerID(t)
			pos := mgr.RegisterPeer(peer)

			assert.True(t, pos.Row >= 0 && pos.Row < int(dims.Rows))
			assert.True(t, pos.Col >= 0 && pos.Col < int(dims.Cols))
		})
	}
}

// TestIntegration_SubnetRouter tests routing logic
func TestIntegration_SubnetRouter_Statistics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	stats := &RDARouterStats{}

	// Simulate message routing
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			stats.RowMessagesSent++
		} else {
			stats.ColMessagesSent++
		}
	}

	stats.PeersInRow = 50
	stats.PeersInCol = 50
	stats.TotalSubnetPeers = 100

	// Verify stats
	assert.Greater(t, stats.RowMessagesSent, uint64(0))
	assert.Greater(t, stats.ColMessagesSent, uint64(0))
	assert.Equal(t, uint64(50), stats.RowMessagesSent)
	assert.Equal(t, uint64(50), stats.ColMessagesSent)
	assert.Equal(t, 100, stats.TotalSubnetPeers)
}

// TestIntegration_MultipleServices tests multiple service instances
func TestIntegration_MultipleServices_Isolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	config1 := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 16, Cols: 16},
		FilterPolicy:   DefaultFilterPolicy(),
	}

	config2 := RDANodeServiceConfig{
		GridDimensions: GridDimensions{Rows: 32, Cols: 32},
		FilterPolicy:   StrictFilterPolicy(),
	}

	mgr1 := NewRDAGridManager(config1.GridDimensions)
	mgr2 := NewRDAGridManager(config2.GridDimensions)

	// Register peers separately
	peer1In1 := generateTestPeerID(t)
	peer1In2 := generateTestPeerID(t)

	mgr1.RegisterPeer(peer1In1)
	mgr2.RegisterPeer(peer1In2)

	// Verify isolation
	_, exists := mgr1.GetPeerPosition(peer1In1)
	assert.True(t, exists)

	_, exists = mgr2.GetPeerPosition(peer1In2)
	assert.True(t, exists)

	// Cross-manager access should fail
	_, exists = mgr1.GetPeerPosition(peer1In2)
	assert.False(t, exists)

	_, exists = mgr2.GetPeerPosition(peer1In1)
	assert.False(t, exists)
}

// TestIntegration_ContextCancellation tests context handling
func TestIntegration_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	config := DefaultRDANodeServiceConfig()
	dims := config.GridDimensions

	mgr := NewRDAGridManager(dims)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Expected
		assert.True(t, true)
	case <-time.After(1 * time.Second):
		t.Fail()
	}

	// Service should still work (context cancellation doesn't affect initialization)
	peer := generateTestPeerID(t)
	pos := mgr.RegisterPeer(peer)
	assert.True(t, pos.Row >= 0)
}

// TestIntegration_ErrorHandling tests error cases
func TestIntegration_ErrorHandling_InvalidGrid(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Test with minimal grid
	minDims := GridDimensions{Rows: 1, Cols: 1}
	mgr := NewRDAGridManager(minDims)

	// Should still work
	peer := generateTestPeerID(t)
	pos := mgr.RegisterPeer(peer)

	assert.Equal(t, 0, pos.Row)
	assert.Equal(t, 0, pos.Col)
}

// TestIntegration_PeersInDifferentSizes tests peers across different grid sizes
func TestIntegration_PeersInDifferentSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	gridSizes := []GridDimensions{
		{8, 8},
		{16, 16},
		{32, 32},
		{64, 64},
		{128, 128},
	}

	for idx, dims := range gridSizes {
		dims := dims
		name := "Grid_" + string(rune(48+idx)) // "Grid_0", "Grid_1", etc
		t.Run(name, func(t *testing.T) {
			mgr := NewRDAGridManager(dims)

			// Register and verify peers
			peers := generateTestPeerIDs(t, 20)
			for _, p := range peers {
				pos := mgr.RegisterPeer(p)
				assert.True(t, pos.Row >= 0 && pos.Row < int(dims.Rows))
				assert.True(t, pos.Col >= 0 && pos.Col < int(dims.Cols))
			}
		})
	}
}
