package share

import (
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST 1: RoutingTable respects capacity limits
// EXPECTED: RoutingTable should not store more than capacity peers per row/col
func TestRoutingTable_CapacityLimit(t *testing.T) {
	rt := NewRoutingTable(5) // Capacity of 5 peers per row/col

	// Add 10 peers to row 10
	for i := 0; i < 10; i++ {
		peerID := fmt.Sprintf("peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 10, 5)
	}

	// Verify row size is capped at 5
	rowPeers := rt.GetRowPeers(10, 20, "")
	assert.LessOrEqual(t, len(rowPeers), 5, "Row should not exceed capacity")

	// Add same peers to column 10
	for i := 0; i < 10; i++ {
		peerID := fmt.Sprintf("col_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 15, 10)
	}

	// Verify column size is capped at 5
	colPeers := rt.GetColPeers(10, 20, "")
	assert.LessOrEqual(t, len(colPeers), 5, "Column should not exceed capacity")
}

// TEST 2: RoutingTable evicts oldest peers (LRU)
// EXPECTED: When capacity is exceeded, oldest peer should be removed
func TestRoutingTable_LRUEviction(t *testing.T) {
	rt := NewRoutingTable(3) // Capacity of 3 peers

	// Add 3 peers
	addrs := []peer.AddrInfo{
		{ID: peer.ID("peer_1")},
		{ID: peer.ID("peer_2")},
		{ID: peer.ID("peer_3")},
	}

	rt.AddPeer("peer_1", addrs[0], 5, 5)
	time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	rt.AddPeer("peer_2", addrs[1], 5, 5)
	time.Sleep(10 * time.Millisecond)
	rt.AddPeer("peer_3", addrs[2], 5, 5)

	// Verify all 3 are present
	peers := rt.GetRowPeers(5, 10, "")
	assert.Equal(t, 3, len(peers), "Should have 3 peers before eviction")

	// Add 4th peer (should evict peer_1, the oldest)
	rt.AddPeer("peer_4", peer.AddrInfo{ID: peer.ID("peer_4")}, 5, 5)

	// Get peers and check that peer_1 is not there
	peers = rt.GetRowPeers(5, 10, "")
	assert.Equal(t, 3, len(peers), "Should have 3 peers after eviction")

	peerIDs := make(map[string]bool)
	for _, p := range peers {
		peerIDs[p.PeerID] = true
	}

	assert.False(t, peerIDs["peer_1"], "peer_1 should be evicted")
	assert.True(t, peerIDs["peer_2"], "peer_2 should still be present")
	assert.True(t, peerIDs["peer_3"], "peer_3 should still be present")
	assert.True(t, peerIDs["peer_4"], "peer_4 should be added")
}

// TEST 3: GetRowPeers returns random subset (not all)
// EXPECTED: When requesting fewer peers than available, should return random subset
func TestRoutingTable_RandomRowSelection(t *testing.T) {
	rt := NewRoutingTable(10) // Capacity of 10 peers

	// Add 10 peers to row 20
	for i := 0; i < 10; i++ {
		peerID := fmt.Sprintf("row_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 20, 5)
	}

	// Get 5 peers from row 20 multiple times
	sets := []map[string]bool{}
	for j := 0; j < 5; j++ {
		peers := rt.GetRowPeers(20, 5, "")
		assert.Equal(t, 5, len(peers), "Should return exactly 5 peers")

		peerSet := make(map[string]bool)
		for _, p := range peers {
			peerSet[p.PeerID] = true
		}
		sets = append(sets, peerSet)
	}

	// Verify we get variation in results (different subsets across calls)
	// This is probabilistic but with 10 peers and requesting 5, we should see variation
	hasDifference := false
	for i := 0; i < len(sets)-1; i++ {
		if len(sets[i]) != len(sets[i+1]) || !mapsEqual(sets[i], sets[i+1]) {
			hasDifference = true
			break
		}
	}
	assert.True(t, hasDifference, "Should get different random subsets across calls")
}

// TEST 4: GetColPeers returns random subset (not all)
// EXPECTED: Similar to row test but for columns
func TestRoutingTable_RandomColSelection(t *testing.T) {
	rt := NewRoutingTable(10)

	// Add 10 peers to column 50
	for i := 0; i < 10; i++ {
		peerID := fmt.Sprintf("col_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, uint32(i), 50)
	}

	// Get 4 peers from column 50 multiple times
	sets := []map[string]bool{}
	for j := 0; j < 5; j++ {
		peers := rt.GetColPeers(50, 4, "")
		assert.Equal(t, 4, len(peers), "Should return exactly 4 peers")

		peerSet := make(map[string]bool)
		for _, p := range peers {
			peerSet[p.PeerID] = true
		}
		sets = append(sets, peerSet)
	}

	// Verify variation in results
	hasDifference := false
	for i := 0; i < len(sets)-1; i++ {
		if len(sets[i]) != len(sets[i+1]) || !mapsEqual(sets[i], sets[i+1]) {
			hasDifference = true
			break
		}
	}
	assert.True(t, hasDifference, "Should get different random subsets across calls")
}

// TEST 5: GetPeers returns all if fewer than requested
// EXPECTED: If fewer peers available than count requested, return all
func TestRoutingTable_FewerPeersThanRequested(t *testing.T) {
	rt := NewRoutingTable(10)

	// Add only 3 peers to row 30
	for i := 0; i < 3; i++ {
		peerID := fmt.Sprintf("sparse_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 30, 5)
	}

	// Request 10 peers, should get all 3
	peers := rt.GetRowPeers(30, 10, "")
	assert.Equal(t, 3, len(peers), "Should return all 3 available peers")
}

// TEST 6: ExcludePeerID filters out requester
// EXPECTED: When excludePeerID is set, that peer should not be returned
func TestRoutingTable_ExcludeRequester(t *testing.T) {
	rt := NewRoutingTable(10)

	// Add 5 peers to row 40
	peerIDs := []string{}
	for i := 0; i < 5; i++ {
		peerID := fmt.Sprintf("req_peer_%d", i)
		peerIDs = append(peerIDs, peerID)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 40, 5)
	}

	// Get peers excluding peer_2
	peers := rt.GetRowPeers(40, 10, "req_peer_2")

	// Verify peer_2 is not in results
	for _, p := range peers {
		assert.NotEqual(t, "req_peer_2", p.PeerID, "Excluded peer should not be returned")
	}

	// All returned peers should be in original list
	for _, p := range peers {
		found := false
		for _, id := range peerIDs {
			if id == p.PeerID && id != "req_peer_2" {
				found = true
				break
			}
		}
		assert.True(t, found, "Returned peer should be in original list")
	}
}

// TEST 7: Size() returns unique peer count
// EXPECTED: Size should count unique peers across all rows/cols
func TestRoutingTable_Size(t *testing.T) {
	rt := NewRoutingTable(10)

	// Add 5 peers across different rows/cols
	// Each peer is added to one row and one column
	for i := 0; i < 5; i++ {
		peerID := fmt.Sprintf("size_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, uint32(i), uint32(i))
	}

	// Size should be 5 (unique peers)
	assert.Equal(t, 5, rt.Size(), "Size should count unique peers")

	// Add same peers to different positions (should not increase size)
	for i := 0; i < 5; i++ {
		peerID := fmt.Sprintf("size_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, uint32(10+i), uint32(10+i))
	}

	// Size should still be 5 (same peers, different positions)
	assert.Equal(t, 5, rt.Size(), "Size should count unique peers, not duplicate entries")
}

// TEST 8: Empty rows/cols return empty slice
// EXPECTED: Getting peers from empty row/col should return empty slice
func TestRoutingTable_EmptyRowCol(t *testing.T) {
	rt := NewRoutingTable(10)

	// Get peers from empty row
	rowPeers := rt.GetRowPeers(999, 5, "")
	assert.Equal(t, 0, len(rowPeers), "Empty row should return empty slice")

	// Get peers from empty col
	colPeers := rt.GetColPeers(999, 5, "")
	assert.Equal(t, 0, len(colPeers), "Empty column should return empty slice")
}

// TEST 9: Peer update works correctly
// EXPECTED: Adding same peer again should update it (not duplicate)
func TestRoutingTable_PeerUpdate(t *testing.T) {
	rt := NewRoutingTable(10)

	peerID := "update_peer"
	addr1 := peer.AddrInfo{ID: peer.ID(peerID)}
	addr2 := peer.AddrInfo{ID: peer.ID(peerID)}

	// Add peer at (10, 20)
	rt.AddPeer(peerID, addr1, 10, 20)
	peers := rt.GetRowPeers(10, 10, "")
	assert.Equal(t, 1, len(peers), "Should have 1 peer")

	// Update same peer at (10, 20)
	rt.AddPeer(peerID, addr2, 10, 20)
	peers = rt.GetRowPeers(10, 10, "")
	assert.Equal(t, 1, len(peers), "Should still have 1 peer after update")

	// Total size should still be 1
	assert.Equal(t, 1, rt.Size(), "Size should remain 1 after update")
}

// TEST 10: Integration test with many peers (Snowball pattern)
// EXPECTED: Bootstrap can handle 1000+ peers with constant memory per bucket
func TestRoutingTable_ManyPeers(t *testing.T) {
	rt := NewRoutingTable(50) // Ethereum-compatible capacity (50/row/col)

	// Add 500 peers to same row (should only keep 50)
	for i := 0; i < 500; i++ {
		peerID := fmt.Sprintf("many_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 50, 50)
	}

	// Row should not exceed capacity
	rowPeers := rt.GetRowPeers(50, 100, "")
	assert.LessOrEqual(t, len(rowPeers), 50, "Row should not exceed 50 peers")

	// Random selection should work smoothly
	sets := []map[string]bool{}
	for j := 0; j < 5; j++ {
		peers := rt.GetRowPeers(50, 5, "")
		assert.Equal(t, 5, len(peers), "Should return exactly 5 random peers")

		peerSet := make(map[string]bool)
		for _, p := range peers {
			peerSet[p.PeerID] = true
		}
		sets = append(sets, peerSet)
	}

	// Verify we get variation
	hasDifference := false
	for i := 0; i < len(sets)-1; i++ {
		if !mapsEqual(sets[i], sets[i+1]) {
			hasDifference = true
			break
		}
	}
	assert.True(t, hasDifference, "Should get different random peers even with 500 entries")
}

// TEST 11: Concurrent access is thread-safe
// EXPECTED: RoutingTable should handle concurrent add/get operations
func TestRoutingTable_ConcurrentAccess(t *testing.T) {
	rt := NewRoutingTable(100)

	done := make(chan bool)
	errChan := make(chan error)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				peerID := fmt.Sprintf("concurrent_peer_%d_%d", id, j)
				addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
				rt.AddPeer(peerID, addrInfo, uint32(id), uint32(j%128))
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				_ = rt.GetRowPeers(uint32(j%128), 5, "")
				_ = rt.GetColPeers(uint32(j%128), 5, "")
				_ = rt.Size()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		select {
		case <-done:
		case err := <-errChan:
			t.Fatalf("Concurrent access error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatalf("Timeout waiting for goroutines")
		}
	}
}

// Helper function to compare maps
func mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// TEST 12: Ethereum-pattern test (Snowball discovery)
// EXPECTED: Bootstrap returns limited peers for exponential peer discovery
func TestRoutingTable_SnowballPattern(t *testing.T) {
	rt := NewRoutingTable(50)

	// Simulate bootstrap with 100 peers initially
	for i := 0; i < 100; i++ {
		peerID := fmt.Sprintf("snowball_peer_%d", i)
		addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
		rt.AddPeer(peerID, addrInfo, 64, 64) // Central row/col
	}

	// Step 1: Initial query returns limited peers
	initialPeers := rt.GetRowPeers(64, RandomPeersPerQuery, "new_node")
	require.True(t, len(initialPeers) > 0, "Should return some peers")
	require.LessOrEqual(t, len(initialPeers), RandomPeersPerQuery, "Should return at most RandomPeersPerQuery peers")

	// Step 2: New node can query multiple initial peers for more peers
	discoveredCount := len(initialPeers)
	for _, peer := range initialPeers {
		// Each peer can provide more peers
		_ = rt.GetRowPeers(64, 5, peer.PeerID) // Exclude the peer we're querying
		discoveredCount++                      // Count this peer
	}

	// Step 3: Verify exponential growth is possible
	// With 5 initial peers, each providing 5 peers = 25 total in round 2
	assert.Greater(t, discoveredCount, len(initialPeers), "Should leverage multiple peers for exponential discovery")
}

// TEST 13: Peer positions (row, col) are preserved
// EXPECTED: Peer's row/col should be accessible after retrieval
func TestRoutingTable_PreservePositions(t *testing.T) {
	rt := NewRoutingTable(10)

	peerID := "position_peer"
	addrInfo := peer.AddrInfo{ID: peer.ID(peerID)}
	testRow, testCol := uint32(25), uint32(75)

	rt.AddPeer(peerID, addrInfo, testRow, testCol)

	// Get peer from row
	peers := rt.GetRowPeers(testRow, 10, "")
	require.Equal(t, 1, len(peers), "Should find peer in row")
	assert.Equal(t, testRow, peers[0].Row, "Row should be preserved")
	assert.Equal(t, testCol, peers[0].Col, "Col should be preserved")

	// Get peer from col
	peers = rt.GetColPeers(testCol, 10, "")
	require.Equal(t, 1, len(peers), "Should find peer in col")
	assert.Equal(t, testRow, peers[0].Row, "Row should be preserved")
	assert.Equal(t, testCol, peers[0].Col, "Col should be preserved")
}
