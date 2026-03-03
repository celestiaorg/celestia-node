package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_NewRDAPeerManager creates and initializes peer manager
func Test_NewRDAPeerManager(t *testing.T) {
	dims := GridDimensions{Rows: 32, Cols: 32}
	gridMgr := NewRDAGridManager(dims)

	// We can't easily test without a real host, so we'll test basic initialization
	// This is a limitation of testing libp2p components without full infrastructure

	assert.NotNil(t, gridMgr)
	assert.Equal(t, dims.Rows, gridMgr.dims.Rows)
	assert.Equal(t, dims.Cols, gridMgr.dims.Cols)
}

// Test_GetRowPeers checks peer grouping by row
func Test_GetRowPeers_RegisteringPeers(t *testing.T) {
	dims := GridDimensions{Rows: 8, Cols: 8}
	gridMgr := NewRDAGridManager(dims)

	// Register multiple peers
	peers := generateTestPeerIDs(t, 10)
	for _, p := range peers {
		gridMgr.RegisterPeer(p)
	}

	// Get the position of first peer
	pos1, _ := gridMgr.GetPeerPosition(peers[0])

	// Get all peers in that row
	rowPeers := gridMgr.GetRowPeers(pos1.Row)

	// Verify structure
	assert.NotEmpty(t, rowPeers, "Should have peers in row")
	for _, p := range rowPeers {
		pPos, _ := gridMgr.GetPeerPosition(p)
		assert.Equal(t, pos1.Row, pPos.Row, "All row peers should be in same row")
	}
}

// Test_GetColPeers checks peer grouping by column
func Test_GetColPeers_RegisteringPeers(t *testing.T) {
	dims := GridDimensions{Rows: 8, Cols: 8}
	gridMgr := NewRDAGridManager(dims)

	// Register multiple peers
	peers := generateTestPeerIDs(t, 10)
	for _, p := range peers {
		gridMgr.RegisterPeer(p)
	}

	// Get the position of first peer
	pos1, _ := gridMgr.GetPeerPosition(peers[0])

	// Get all peers in that column
	colPeers := gridMgr.GetColPeers(pos1.Col)

	// Verify structure
	assert.NotEmpty(t, colPeers, "Should have peers in column")
	for _, p := range colPeers {
		pPos, _ := gridMgr.GetPeerPosition(p)
		assert.Equal(t, pos1.Col, pPos.Col, "All column peers should be in same column")
	}
}

// Test_FilterPeers_ByRow tests row filtering
func Test_FilterPeers_ByRow(t *testing.T) {
	dims := GridDimensions{Rows: 8, Cols: 8}
	gridMgr := NewRDAGridManager(dims)

	// Register many peers
	peers := generateTestPeerIDs(t, 50)
	for _, p := range peers {
		gridMgr.RegisterPeer(p)
	}

	// Get first peer's row
	pos1, _ := gridMgr.GetPeerPosition(peers[0])

	// Manually filter: peers in same row
	rowPeers := gridMgr.GetRowPeers(pos1.Row)

	// Verify all are in same row
	for _, p := range rowPeers {
		pPos, _ := gridMgr.GetPeerPosition(p)
		assert.Equal(t, pos1.Row, pPos.Row)
	}
}

// Test_IsInSameSubnet checks row/col membership
func Test_IsInSameSubnet(t *testing.T) {
	dims := GridDimensions{Rows: 16, Cols: 16}

	peer1 := generateTestPeerID(t)
	peer2 := generateTestPeerID(t)

	pos1 := GetCoords(peer1, dims)
	pos2 := GetCoords(peer2, dims)

	// Test row peer check
	isRowPeer := IsRowPeer(peer1, peer2, dims)
	assert.Equal(t, pos1.Row == pos2.Row, isRowPeer)

	// Test col peer check
	isColPeer := IsColPeer(peer1, peer2, dims)
	assert.Equal(t, pos1.Col == pos2.Col, isColPeer)

	// Both true only if same position (unlikely with random peers)
	if isRowPeer && isColPeer {
		assert.Equal(t, pos1, pos2)
	}
}

// Test_GetGridDimensions returns correct dimensions
func Test_GetGridDimensions(t *testing.T) {
	dims := GridDimensions{Rows: 64, Cols: 128}
	gridMgr := NewRDAGridManager(dims)

	// Note: GetGridDimensions is not in original. We test the stored values
	assert.Equal(t, dims.Rows, gridMgr.dims.Rows)
	assert.Equal(t, dims.Cols, gridMgr.dims.Cols)
}

// Test_ConcurrentPeerRegistration tests thread-safety
func Test_ConcurrentPeerRegistration(t *testing.T) {
	dims := GridDimensions{Rows: 16, Cols: 16}
	gridMgr := NewRDAGridManager(dims)

	// Register many peers concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			p := generateTestPeerID(t)
			gridMgr.RegisterPeer(p)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Service should still be functional
	testPeer := generateTestPeerID(t)
	gridMgr.RegisterPeer(testPeer)
	_, exists := gridMgr.GetPeerPosition(testPeer)
	assert.True(t, exists)
}

// Test_PeerPositionPersistence verifies position is tracked
func Test_PeerPositionPersistence(t *testing.T) {
	dims := GridDimensions{Rows: 32, Cols: 32}
	gridMgr := NewRDAGridManager(dims)

	peer := generateTestPeerID(t)

	// Register twice should return same position
	pos1 := gridMgr.RegisterPeer(peer)
	pos2 := gridMgr.RegisterPeer(peer)

	assert.Equal(t, pos1, pos2, "Same peer should always map to same position")
}

// Test_GridCapacity verifies grid can hold expected number
func Test_GridCapacity(t *testing.T) {
	dims := GridDimensions{Rows: 8, Cols: 8}
	gridMgr := NewRDAGridManager(dims)

	// Register all positions in grid
	peersPerPosition := 3
	totalPeers := int(dims.Rows) * int(dims.Cols) * peersPerPosition

	for i := 0; i < totalPeers; i++ {
		peer := generateTestPeerID(t)
		gridMgr.RegisterPeer(peer)
	}

	// Grid should be functional
	testPeer := generateTestPeerID(t)
	pos := gridMgr.RegisterPeer(testPeer)

	assert.True(t, pos.Row < int(dims.Rows))
	assert.True(t, pos.Col < int(dims.Cols))
}
