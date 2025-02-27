package core

import (
	"fmt"
	"testing"

	"google.golang.org/grpc"
)

// newTestBlockFetcher creates a BlockFetcher for testing using a single connection.
// This is a helper function for creating a fetcher with a single endpoint in tests.
func newTestBlockFetcher(t *testing.T, host, port string) (*BlockFetcher, error) {
	client := newTestClient(t, host, port)
	if client == nil {
		return nil, fmt.Errorf("failed to create test client for %s:%s", host, port)
	}
	conn := []*grpc.ClientConn{client}
	return NewMultiBlockFetcher(conn)
}

// newTestMultiBlockFetcher creates a BlockFetcher for testing using multiple connections.
// This is useful for testing round-robin behavior across multiple endpoints.
func newTestMultiBlockFetcher(t *testing.T, endpoints []struct{ host, port string }) (*BlockFetcher, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("at least one endpoint is required")
	}

	conns := make([]*grpc.ClientConn, 0, len(endpoints))
	for _, ep := range endpoints {
		client := newTestClient(t, ep.host, ep.port)
		if client == nil {
			// Clean up any connections we've already created
			for _, conn := range conns {
				conn.Close()
			}
			return nil, fmt.Errorf("failed to create test client for %s:%s", ep.host, ep.port)
		}
		conns = append(conns, client)
	}

	return NewMultiBlockFetcher(conns)
}
