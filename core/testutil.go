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
