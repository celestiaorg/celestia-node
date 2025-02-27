//go:build !race

package state

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// TestRoundRobinConnectionSelection tests that the getNextConn method
// properly rotates through all available connections in a round-robin fashion
func TestRoundRobinConnectionSelection(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Create multiple mock connections
	connCount := 3
	conns := make([]*grpc.ClientConn, connCount)
	for i := 0; i < connCount; i++ {
		// We just need the pointer, not an actual working connection
		conns[i] = &grpc.ClientConn{}
	}

	// Create a minimal CoreAccessor with only the fields needed for the test
	ca := &CoreAccessor{
		coreConns:     conns,
		nextConnIndex: atomic.Uint64{}, // Initialize zero value directly
	}
	// Initialize the counter to start from the beginning
	ca.nextConnIndex.Store(0)

	// Call getNextConn multiple times and verify the connection rotation
	for i := 0; i < connCount*2; i++ {
		expectedConnIndex := i % connCount
		conn := ca.getNextConn()

		// The current connection should match the expected one based on round-robin
		require.Equal(t, conns[expectedConnIndex], conn,
			"Call %d should return connection %d", i, expectedConnIndex)
	}
}
