package core

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
	"google.golang.org/grpc"
)

// TestNewMultiBlockFetcher tests the NewMultiBlockFetcher function
// directly to ensure it properly creates a BlockFetcher with multiple connections.
func TestNewMultiBlockFetcher(t *testing.T) {
	// Create test connections
	conn1 := &grpc.ClientConn{}
	conn2 := &grpc.ClientConn{}
	conn3 := &grpc.ClientConn{}

	conns := []*grpc.ClientConn{conn1, conn2, conn3}

	// Create fetcher with multiple connections
	fetcher, err := NewMultiBlockFetcher(conns)
	require.NoError(t, err)
	require.NotNil(t, fetcher)

	// Verify the clients were created
	assert.Equal(t, len(conns), len(fetcher.clients))

	// Test error case with empty connections
	_, err = NewMultiBlockFetcher([]*grpc.ClientConn{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one connection is required")
}

// TestBlockFetcherRoundRobin tests the round-robin client selection mechanism
// of the BlockFetcher by creating a fetcher with custom mock clients.
func TestBlockFetcherRoundRobin(t *testing.T) {
	// Create mock clients
	clients := []coregrpc.BlockAPIClient{
		&mockBlockAPIClient{id: 1},
		&mockBlockAPIClient{id: 2},
		&mockBlockAPIClient{id: 3},
	}

	// Create fetcher with mock clients
	fetcher := &BlockFetcher{
		clients:       clients,
		currentClient: atomic.Uint32{},
	}

	// Test round-robin rotation
	for i := 0; i < 6; i++ {
		client := fetcher.getNextClient()
		mockClient, ok := client.(*mockBlockAPIClient)
		require.True(t, ok, "Expected mockBlockAPIClient")

		// Expected client ID should rotate through 1, 2, 3, 1, 2, 3
		expectedID := (i % 3) + 1
		assert.Equal(t, expectedID, mockClient.id, "Incorrect client selected at iteration %d", i)
	}
}
