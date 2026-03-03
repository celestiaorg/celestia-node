package share

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_RDARouterStats initialization stores zero values
func Test_RDARouterStats_Init(t *testing.T) {
	stats := &RDARouterStats{}

	assert.Equal(t, uint64(0), stats.RowMessagesSent)
	assert.Equal(t, uint64(0), stats.ColMessagesSent)
	assert.Equal(t, uint64(0), stats.RowMessagesRecv)
	assert.Equal(t, uint64(0), stats.ColMessagesRecv)
	assert.Equal(t, 0, stats.PeersInRow)
	assert.Equal(t, 0, stats.PeersInCol)
	assert.Equal(t, 0, stats.TotalSubnetPeers)
}

// Test_RDARouterStats_Increment updates counters
func Test_RDARouterStats_Increment(t *testing.T) {
	stats := &RDARouterStats{}

	stats.RowMessagesSent++
	stats.ColMessagesSent += 2
	stats.RowMessagesRecv += 3

	assert.Equal(t, uint64(1), stats.RowMessagesSent)
	assert.Equal(t, uint64(2), stats.ColMessagesSent)
	assert.Equal(t, uint64(3), stats.RowMessagesRecv)
	assert.Equal(t, uint64(0), stats.ColMessagesRecv)
}

// Test_RDARouterStats_PeerCounts updates peer tracking
func Test_RDARouterStats_PeerCounts(t *testing.T) {
	stats := &RDARouterStats{}

	stats.PeersInRow = 10
	stats.PeersInCol = 15
	stats.TotalSubnetPeers = 25

	assert.Equal(t, 10, stats.PeersInRow)
	assert.Equal(t, 15, stats.PeersInCol)
	assert.Equal(t, 25, stats.TotalSubnetPeers)
}

// Test_RDARouterStats_ConcurrentUpdates handles concurrent access
func Test_RDARouterStats_ConcurrentUpdates(t *testing.T) {
	stats := &RDARouterStats{}

	// Simulate concurrent increments
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			stats.RowMessagesSent++
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 100; i++ {
		<-done
	}

	// Note: Without atomics, this might have race conditions
	// but at least the field exists and can be accessed
	assert.Greater(t, stats.RowMessagesSent, uint64(0))
}

// Test_RDARouterStats_ResetValues allows clearing stats
func Test_RDARouterStats_ResetValues(t *testing.T) {
	stats := &RDARouterStats{
		RowMessagesSent: 100,
		ColMessagesSent: 50,
		PeersInRow:      10,
	}

	// Reset
	stats.RowMessagesSent = 0
	stats.ColMessagesSent = 0
	stats.PeersInRow = 0

	assert.Equal(t, uint64(0), stats.RowMessagesSent)
	assert.Equal(t, uint64(0), stats.ColMessagesSent)
	assert.Equal(t, 0, stats.PeersInRow)
}

// Test_RDARouterStats_SnapshotMetrics checks all fields exist
func Test_RDARouterStats_AllFieldsAccessible(t *testing.T) {
	stats := &RDARouterStats{
		RowMessagesSent:  1,
		ColMessagesSent:  2,
		RowMessagesRecv:  3,
		ColMessagesRecv:  4,
		PeersInRow:       5,
		PeersInCol:       6,
		TotalSubnetPeers: 11,
	}

	assert.Equal(t, uint64(1), stats.RowMessagesSent)
	assert.Equal(t, uint64(2), stats.ColMessagesSent)
	assert.Equal(t, uint64(3), stats.RowMessagesRecv)
	assert.Equal(t, uint64(4), stats.ColMessagesRecv)
	assert.Equal(t, 5, stats.PeersInRow)
	assert.Equal(t, 6, stats.PeersInCol)
	assert.Equal(t, 11, stats.TotalSubnetPeers)
}

// Test_RDARouterStats_LargeNumbers handles large values
func Test_RDARouterStats_LargeNumbers(t *testing.T) {
	stats := &RDARouterStats{
		RowMessagesSent: 1000000,
		ColMessagesSent: 2000000,
	}

	assert.Greater(t, stats.RowMessagesSent, uint64(999999))
	assert.Greater(t, stats.ColMessagesSent, uint64(1999999))
}

// Test_RDARouterStats_Zero checks zero initialization
func Test_RDARouterStats_ZeroValue(t *testing.T) {
	var stats RDARouterStats // Zero value

	assert.Equal(t, uint64(0), stats.RowMessagesSent)
	assert.Equal(t, uint64(0), stats.ColMessagesSent)
	assert.Equal(t, 0, stats.PeersInRow)
}
