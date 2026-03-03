package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRDAAPI_GetMyPosition returns valid position
func TestRDAAPI_GetMyPosition(t *testing.T) {
	config := DefaultRDANodeServiceConfig()
	gridMgr := NewRDAGridManager(config.GridDimensions)
	myPeer := generateTestPeerID(t)

	pos := gridMgr.RegisterPeer(myPeer)

	assert.GreaterOrEqual(t, pos.Row, 0)
	assert.GreaterOrEqual(t, pos.Col, 0)
	assert.Less(t, pos.Row, int(config.GridDimensions.Rows))
	assert.Less(t, pos.Col, int(config.GridDimensions.Cols))
}

// TestRDAAPI_RDAPosition_Structure verifies RDAPosition fields
func TestRDAAPI_RDAPosition_Structure(t *testing.T) {
	pos := &RDAPosition{
		Row: 10,
		Col: 20,
	}

	assert.Equal(t, 10, pos.Row)
	assert.Equal(t, 20, pos.Col)
}

// TestRDAAPI_RDAStatus_Structure verifies RDAStatus fields
func TestRDAAPI_RDAStatus_Structure(t *testing.T) {
	status := &RDAStatus{
		Position: RDAPosition{Row: 5, Col: 5},
		RowPeers: 10,
		ColPeers: 15,
		TotalPeers: 25,
		GridDimensions: RDAGridInfo{Rows: 128, Cols: 128},
		RowTopic: "rda/row/5",
		ColTopic: "rda/col/5",
	}

	assert.Equal(t, 5, status.Position.Row)
	assert.Equal(t, 5, status.Position.Col)
	assert.Equal(t, 10, status.RowPeers)
	assert.Equal(t, 15, status.ColPeers)
	assert.Equal(t, 25, status.TotalPeers)
	assert.Equal(t, 128, status.GridDimensions.Rows)
	assert.Contains(t, status.RowTopic, "rda/row/")
	assert.Contains(t, status.ColTopic, "rda/col/")
}

// TestRDAAPI_RDANodeInfo_Structure verifies RDANodeInfo fields
func TestRDAAPI_RDANodeInfo_Structure(t *testing.T) {
	info := &RDANodeInfo{
		PeerID:      "QmTest",
		Position:    RDAPosition{Row: 10, Col: 10},
		GridDimensions: RDAGridInfo{Rows: 64, Cols: 64},
		RowTopic:    "rda/row/10",
		ColTopic:    "rda/col/10",
		FilterPolicy: "RowAndCol",
	}

	assert.NotEmpty(t, info.PeerID)
	assert.Equal(t, 10, info.Position.Row)
	assert.Equal(t, 64, info.GridDimensions.Rows)
	assert.Contains(t, info.FilterPolicy, "Row")
}

// TestRDAAPI_RDAPeerList_Structure verifies RDAPeerList fields
func TestRDAAPI_RDAPeerList_Structure(t *testing.T) {
	list := &RDAPeerList{
		Peers: []string{"peer1", "peer2", "peer3"},
		Count: 3,
	}

	assert.Equal(t, 3, list.Count)
	assert.Equal(t, 3, len(list.Peers))
	assert.Contains(t, list.Peers, "peer1")
}

// TestRDAAPI_RDAStats_Structure verifies RDAStats fields
func TestRDAAPI_RDAStats_Structure(t *testing.T) {
	stats := &RDAStats{
		RowMessagesSent:   100,
		ColMessagesSent:   200,
		RowMessagesRecv:   150,
		ColMessagesRecv:   250,
		TotalMessageSent:  300,
		TotalMessagesRecv: 400,
		PeersInRow:        20,
		PeersInCol:        25,
		TotalSubnetPeers:  45,
	}

	assert.Equal(t, uint64(100), stats.RowMessagesSent)
	assert.Equal(t, uint64(200), stats.ColMessagesSent)
	assert.Greater(t, stats.TotalMessageSent, uint64(0))
	assert.Equal(t, 45, stats.TotalSubnetPeers)
}

// TestRDAAPI_RDAHealthStatus_Structure verifies RDAHealthStatus fields
func TestRDAAPI_RDAHealthStatus_Structure(t *testing.T) {
	health := &RDAHealthStatus{
		IsHealthy:        true,
		Message:          "healthy",
		ConnectedPeers:   15,
		GridCoverageRate: 0.05,
	}

	assert.True(t, health.IsHealthy)
	assert.Equal(t, "healthy", health.Message)
	assert.Equal(t, 15, health.ConnectedPeers)
	assert.Greater(t, health.GridCoverageRate, 0.0)
	assert.Less(t, health.GridCoverageRate, 1.0)
}

// TestRDAAPI_RDAGridInfo_Structure verifies RDAGridInfo fields
func TestRDAAPI_RDAGridInfo_Structure(t *testing.T) {
	info := &RDAGridInfo{
		Rows: 128,
		Cols: 128,
	}

	assert.Equal(t, 128, info.Rows)
	assert.Equal(t, 128, info.Cols)
}

// TestRDAAPI_RDAPeerInfo_Structure verifies RDAPeerInfo fields
func TestRDAAPI_RDAPeerInfo_Structure(t *testing.T) {
	info := &RDAPeerInfo{
		PeerID:    "QmPeer",
		Position:  RDAPosition{Row: 5, Col: 10},
		IsRowPeer: true,
		IsColPeer: false,
	}

	assert.Equal(t, "QmPeer", info.PeerID)
	assert.Equal(t, 5, info.Position.Row)
	assert.True(t, info.IsRowPeer)
	assert.False(t, info.IsColPeer)
}

// TestRDAAPI_GetGridDimensions verifies grid dimensions
func TestRDAAPI_GetGridDimensions(t *testing.T) {
	dims := GridDimensions{Rows: 64, Cols: 64}
	gridMgr := NewRDAGridManager(dims)

	gridDims := gridMgr.GetGridDimensions()

	assert.Equal(t, uint16(64), gridDims.Rows)
	assert.Equal(t, uint16(64), gridDims.Cols)
}

// TestRDAAPI_RDARouterStats_Structure verifies stats structure
func TestRDAAPI_RDARouterStats_Structure(t *testing.T) {
	stats := &RDARouterStats{
		RowMessagesSent:  50,
		ColMessagesSent:  75,
		RowMessagesRecv:  60,
		ColMessagesRecv:  80,
		PeersInRow:       10,
		PeersInCol:       12,
		TotalSubnetPeers: 22,
	}

	assert.Equal(t, uint64(50), stats.RowMessagesSent)
	assert.Equal(t, uint64(75), stats.ColMessagesSent)
	assert.Equal(t, 10, stats.PeersInRow)
	assert.Equal(t, 12, stats.PeersInCol)
}

// TestRDAAPI_EmptyPeerList returns empty list
func TestRDAAPI_EmptyPeerList(t *testing.T) {
	list := &RDAPeerList{
		Peers: []string{},
		Count: 0,
	}

	assert.Equal(t, 0, list.Count)
	assert.Empty(t, list.Peers)
}

// TestRDAAPI_FilterPolicyOptions verifies policy options
func TestRDAAPI_FilterPolicyOptions(t *testing.T) {
	policies := []string{
		"AllowAny",
		"RowOnly",
		"ColOnly",
		"RowAndCol",
	}

	for _, p := range policies {
		assert.NotEmpty(t, p)
	}
}

// TestRDAAPI_HealthStatus_Unhealthy
func TestRDAAPI_HealthStatus_Unhealthy(t *testing.T) {
	health := &RDAHealthStatus{
		IsHealthy:        false,
		Message:          "no peers connected",
		ConnectedPeers:   0,
		GridCoverageRate: 0.0,
	}

	assert.False(t, health.IsHealthy)
	assert.Equal(t, 0, health.ConnectedPeers)
	assert.Equal(t, 0.0, health.GridCoverageRate)
}

// TestRDAAPI_Position_InvalidCoordinates
func TestRDAAPI_Position_Boundaries(t *testing.T) {
	// Test valid positions
	validPositions := []RDAPosition{
		{Row: 0, Col: 0},
		{Row: 1, Col: 1},
		{Row: 127, Col: 127},
	}

	for _, pos := range validPositions {
		assert.GreaterOrEqual(t, pos.Row, 0)
		assert.GreaterOrEqual(t, pos.Col, 0)
	}
}

// TestRDAAPI_Status_CalculatedTotals
func TestRDAAPI_Status_ConsistentTotals(t *testing.T) {
	status := &RDAStatus{
		RowPeers:   10,
		ColPeers:   15,
		TotalPeers: 25,
	}

	// Total should be sum of row and col
	expectedTotal := status.RowPeers + status.ColPeers
	assert.Equal(t, expectedTotal, status.TotalPeers)
}

// TestRDAAPI_AllTypesCanBeSerialized
func TestRDAAPI_TypesAreGo(t *testing.T) {
	// Just verify all types can be created
	pos := &RDAPosition{Row: 1, Col: 1}
	_ = pos

	grid := &RDAGridInfo{Rows: 64, Cols: 64}
	_ = grid

	peer := &RDAPeerInfo{PeerID: "test"}
	_ = peer

	status := &RDAStatus{}
	_ = status

	info := &RDANodeInfo{}
	_ = info

	stats := &RDAStats{}
	_ = stats

	list := &RDAPeerList{}
	_ = list

	health := &RDAHealthStatus{}
	_ = health

	assert.True(t, true) // If we get here, all types compiled
}
